import asyncio
import base64
import json
import sqlite3
import threading
import time
import uuid
from datetime import datetime, timedelta
from pathlib import Path

from conf import BASE_DIR
from uploader.douyin_uploader.main import DouYinVideo
from uploader.ks_uploader.main import KSVideo
from uploader.tencent_uploader.main import TencentVideo
from utils.constant import TencentZoneTypes
from utils.files_times import generate_schedule_time_next_day
from utils.log import log_throttled, task_logger
from utils.materials import resolve_material_reference
from utils.publish_verification import PublishManualVerificationRequired


PLATFORM_LABELS = {
    1: "小红书",
    2: "视频号",
    3: "抖音",
    4: "快手",
}

FINISHED_STATUSES = {
    "success",
    "failed",
    "needs_verify",
    "cancelled",
}


class PublishTaskManager:
    def __init__(self, db_path, worker_count=2, retention_days=7, sync_client=None, material_roots=None):
        self.db_path = Path(db_path)
        self.worker_count = max(1, int(worker_count))
        self.retention_days = max(1, int(retention_days))
        self.sync_client = sync_client
        self.material_roots = material_roots or {}

        self._started = False
        self._start_lock = threading.Lock()
        self._stop_event = threading.Event()
        self._workers = []
        self._account_locks = {}
        self._account_locks_lock = threading.Lock()
        self._artifact_dir = Path(BASE_DIR / "taskArtifacts" / "publish_verify")
        self._artifact_dir.mkdir(parents=True, exist_ok=True)

    def start(self):
        with self._start_lock:
            if self._started:
                return

            self.init_db()
            self._stop_event.clear()

            for index in range(self.worker_count):
                worker = threading.Thread(
                    target=self._worker_loop,
                    args=(f"worker-{index + 1}",),
                    daemon=True,
                )
                worker.start()
                self._workers.append(worker)

            cleanup_worker = threading.Thread(target=self._cleanup_loop, daemon=True)
            cleanup_worker.start()
            self._workers.append(cleanup_worker)
            self._started = True
            task_logger.info(
                "publish task manager started worker_count={} retention_days={} db_path={}",
                self.worker_count,
                self.retention_days,
                self.db_path,
            )

    def init_db(self):
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                '''
                CREATE TABLE IF NOT EXISTS publish_tasks (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    task_uuid TEXT NOT NULL UNIQUE,
                    source TEXT NOT NULL DEFAULT 'local_api',
                    platform_type INTEGER NOT NULL,
                    platform_name TEXT NOT NULL,
                    account_name TEXT NOT NULL,
                    account_file_path TEXT NOT NULL,
                    file_name TEXT NOT NULL,
                    file_path TEXT NOT NULL,
                    title TEXT NOT NULL,
                    run_at DATETIME,
                    platform_publish_at DATETIME,
                    status TEXT NOT NULL DEFAULT 'pending',
                    message TEXT,
                    payload_json TEXT NOT NULL,
                    verification_data TEXT,
                    artifact_path TEXT,
                    worker_name TEXT,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    started_at DATETIME,
                    finished_at DATETIME
                )
                '''
            )
            cursor.execute(
                '''
                UPDATE publish_tasks
                SET status = 'failed',
                    message = 'OmniBull 重启导致任务中断，请按需重试',
                    finished_at = COALESCE(finished_at, CURRENT_TIMESTAMP),
                    updated_at = CURRENT_TIMESTAMP
                WHERE status = 'running'
                '''
            )
            conn.commit()

    def enqueue_from_request(self, data, source="local_api"):
        tasks = self._build_task_specs(data, source=source)
        return self.enqueue_specs(tasks)

    def enqueue_specs(self, tasks):
        inserted = []

        with self._connect() as conn:
            cursor = conn.cursor()
            for task in tasks:
                cursor.execute(
                    '''
                    INSERT INTO publish_tasks (
                        task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                        file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                    )
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    ''',
                    (
                        task["taskUuid"],
                        task["source"],
                        task["platformType"],
                        task["platformName"],
                        task["accountName"],
                        task["accountFilePath"],
                        task["fileName"],
                        task["filePath"],
                        task["title"],
                        task["runAt"],
                        task["platformPublishAt"],
                        task["status"],
                        task["message"],
                        json.dumps(task["payload"], ensure_ascii=False),
                    ),
                )
                inserted.append(task)
            conn.commit()

        for task in inserted:
            self._sync_task(task["taskUuid"])

        if inserted:
            task_logger.info(
                "publish tasks enqueued count={} sources={}",
                len(inserted),
                ",".join(sorted({str(task.get("source") or "local_api") for task in inserted})),
            )
        return inserted

    def list_tasks(self, limit=100, status=None, source=None, sources=None):
        limit = max(1, min(int(limit), 500))
        query = "SELECT * FROM publish_tasks"
        params = []
        conditions = []
        if status:
            conditions.append("status = ?")
            params.append(status)
        source_values = [str(item).strip() for item in (sources or []) if str(item).strip()]
        if source:
            source_values.append(str(source).strip())
        if source_values:
            placeholders = ",".join("?" for _ in source_values)
            conditions.append(f"source IN ({placeholders})")
            params.extend(source_values)
        if conditions:
            query += " WHERE " + " AND ".join(conditions)
        query += " ORDER BY created_at DESC, id DESC LIMIT ?"
        params.append(limit)

        with self._connect() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(query, params)
            rows = cursor.fetchall()
        return [self._serialize_row(row) for row in rows]

    def get_task(self, task_uuid):
        with self._connect() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                '''
                SELECT * FROM publish_tasks
                WHERE task_uuid = ?
                ''',
                (task_uuid,),
            )
            row = cursor.fetchone()
        return self._serialize_row(row) if row else None

    def cancel_task_if_queued(self, task_uuid, message="本地任务已取消"):
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                '''
                UPDATE publish_tasks
                SET status = 'cancelled',
                    message = ?,
                    finished_at = CURRENT_TIMESTAMP,
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                  AND status IN ('pending', 'scheduled')
                ''',
                (message, task_uuid),
            )
            changed = cursor.rowcount == 1
            conn.commit()
        if changed:
            self._sync_task(task_uuid)
        return changed

    def _build_task_specs(self, data, source="local_api"):
        platform_type = int(data.get("type"))
        platform_name = PLATFORM_LABELS.get(platform_type)
        if not platform_name:
            raise ValueError(f"不支持的平台类型: {platform_type}")

        title = str(data.get("title") or "").strip()
        if not title:
            raise ValueError("标题不能为空")

        tags = data.get("tags") or []
        if not isinstance(tags, list):
            tags = [str(tags)]

        file_inputs = self._normalize_file_inputs(data)
        account_paths = [str(path).strip() for path in (data.get("accountList") or []) if str(path).strip()]

        if not file_inputs:
            raise ValueError("文件列表不能为空")
        if not account_paths:
            raise ValueError("账号列表不能为空")

        file_lookup = self._load_file_records([
            file_input["filePath"]
            for file_input in file_inputs
            if file_input["mode"] == "video_record"
        ])
        account_lookup = self._load_account_records(account_paths)

        enable_timer = bool(int(data.get("enableTimer") or 0))
        videos_per_day = int(data.get("videosPerDay") or 1)
        start_days = int(data.get("startDays") or 0)
        daily_times = data.get("dailyTimes") or None
        publish_dates = (
            generate_schedule_time_next_day(len(file_inputs), videos_per_day, daily_times, start_days=start_days)
            if enable_timer else
            [0 for _ in file_inputs]
        )

        run_at = self._normalize_datetime(data.get("runAt") or data.get("executeAt"))
        task_status = "scheduled" if self._is_future_datetime(run_at) else "pending"
        task_message = "等待定时执行" if task_status == "scheduled" else "等待执行"

        thumbnail_item = self._normalize_thumbnail_item(data.get("thumbnailItem"))
        thumbnail_payload = None
        if thumbnail_item:
            thumbnail_payload = {
                "thumbnailSourceMode": "material",
                "thumbnailRoot": thumbnail_item["root"],
                "thumbnailPath": thumbnail_item["path"],
                "thumbnailAbsolutePath": thumbnail_item["absolutePath"],
            }
        elif data.get("thumbnail"):
            thumbnail_payload = {
                "thumbnailSourceMode": "video_record",
                "thumbnailPath": data.get("thumbnail"),
            }

        specs = []
        for file_index, file_input in enumerate(file_inputs):
            if file_input["mode"] == "video_record":
                file_record = file_lookup.get(file_input["filePath"], {})
                file_name = file_record.get("filename") or self._display_file_name(file_input["filePath"])
                stored_file_path = file_input["filePath"]
            else:
                file_name = file_input["displayName"]
                stored_file_path = file_input["displayPath"]
            publish_date = publish_dates[file_index]
            publish_date_str = self._normalize_datetime(publish_date)

            for account_file_path in account_paths:
                account_record = account_lookup.get(account_file_path, {})
                account_name = account_record.get("userName") or account_file_path
                task_uuid = uuid.uuid4().hex
                payload = {
                    "platformType": platform_type,
                    "platformName": platform_name,
                    "title": title,
                    "tags": tags,
                    "filePath": file_input.get("filePath") or stored_file_path,
                    "accountFilePath": account_file_path,
                    "accountName": account_name,
                    "publishDate": publish_date_str or 0,
                    "category": data.get("category"),
                    "fileSourceMode": file_input["mode"],
                    "materialRoot": file_input.get("root"),
                    "materialPath": file_input.get("path"),
                    "sourceAbsolutePath": file_input.get("absolutePath"),
                    "productLink": data.get("productLink") or "",
                    "productTitle": data.get("productTitle") or "",
                    "isDraft": bool(data.get("isDraft", False)),
                    "source": source,
                }
                if thumbnail_payload:
                    payload.update(thumbnail_payload)
                specs.append(
                    {
                        "taskUuid": task_uuid,
                        "source": source,
                        "platformType": platform_type,
                        "platformName": platform_name,
                        "accountName": account_name,
                        "accountFilePath": account_file_path,
                        "fileName": file_name,
                        "filePath": stored_file_path,
                        "title": title,
                        "runAt": run_at,
                        "platformPublishAt": publish_date_str,
                        "status": task_status,
                        "message": task_message,
                        "payload": payload,
                    }
                )
        return specs

    def _load_file_records(self, file_paths):
        if not file_paths:
            return {}

        placeholders = ",".join("?" for _ in file_paths)
        try:
            with self._connect() as conn:
                conn.row_factory = sqlite3.Row
                cursor = conn.cursor()
                cursor.execute(
                    f'''
                    SELECT * FROM file_records
                    WHERE file_path IN ({placeholders})
                    ''',
                    file_paths,
                )
                rows = cursor.fetchall()
        except sqlite3.OperationalError:
            return {}
        return {row["file_path"]: dict(row) for row in rows}

    def _load_account_records(self, account_paths):
        if not account_paths:
            return {}

        placeholders = ",".join("?" for _ in account_paths)
        try:
            with self._connect() as conn:
                conn.row_factory = sqlite3.Row
                cursor = conn.cursor()
                cursor.execute(
                    f'''
                    SELECT * FROM user_info
                    WHERE filePath IN ({placeholders})
                    ''',
                    account_paths,
                )
                rows = cursor.fetchall()
        except sqlite3.OperationalError:
            return {}
        return {row["filePath"]: dict(row) for row in rows}

    def _worker_loop(self, worker_name):
        task_logger.debug("publish worker started worker_name={}", worker_name)
        while not self._stop_event.is_set():
            task = self._claim_next_ready_task(worker_name)
            if not task:
                time.sleep(1)
                continue

            account_lock = self._get_account_lock(task["account_file_path"])
            with account_lock:
                self._run_task(task)

    def _cleanup_loop(self):
        while not self._stop_event.is_set():
            self._cleanup_old_tasks()
            self._stop_event.wait(3600)

    def _cleanup_old_tasks(self):
        cutoff = (datetime.now() - timedelta(days=self.retention_days)).strftime("%Y-%m-%d %H:%M:%S")
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                f'''
                DELETE FROM publish_tasks
                WHERE status IN ({",".join("?" for _ in FINISHED_STATUSES)})
                  AND updated_at < ?
                ''',
                [*FINISHED_STATUSES, cutoff],
            )
            deleted_count = cursor.rowcount
            conn.commit()
        if deleted_count:
            task_logger.debug(
                "publish task cleanup deleted_count={} retention_days={}",
                deleted_count,
                self.retention_days,
            )

    def _claim_next_ready_task(self, worker_name):
        ready_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")

        with self._connect() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute("BEGIN IMMEDIATE")
            cursor.execute(
                '''
                SELECT * FROM publish_tasks
                WHERE status IN ('pending', 'scheduled')
                  AND (run_at IS NULL OR run_at = '' OR run_at <= ?)
                ORDER BY CASE WHEN run_at IS NULL OR run_at = '' THEN 0 ELSE 1 END,
                         run_at ASC,
                         created_at ASC,
                         id ASC
                LIMIT 1
                ''',
                (ready_at,),
            )
            row = cursor.fetchone()
            if not row:
                conn.commit()
                return None

            cursor.execute(
                '''
                UPDATE publish_tasks
                SET status = 'running',
                    message = '任务执行中',
                    worker_name = ?,
                    started_at = COALESCE(started_at, CURRENT_TIMESTAMP),
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                  AND status IN ('pending', 'scheduled')
                ''',
                (worker_name, row["task_uuid"]),
            )
            if cursor.rowcount != 1:
                conn.commit()
                return None
            conn.commit()

        task = self.get_task(row["task_uuid"])
        self._sync_task(row["task_uuid"])
        task_logger.debug(
            "publish task claimed task_uuid={} worker_name={} platform={} account_name={}",
            row["task_uuid"],
            worker_name,
            row["platform_name"],
            row["account_name"],
        )
        return task

    def _run_task(self, task):
        payload = task["payload"] or {}
        task_uuid = task["taskUuid"]
        task_logger.info(
            "publish task started task_uuid={} platform={} account_name={} title={}",
            task_uuid,
            payload.get("platformName"),
            payload.get("accountName"),
            payload.get("title"),
        )
        try:
            self._execute_payload(payload)
            self._update_task(
                task_uuid,
                status="success",
                message="发布任务执行成功",
                finished=True,
            )
            task_logger.info("publish task succeeded task_uuid={}", task_uuid)
        except PublishManualVerificationRequired as exc:
            verification_payload = dict(exc.payload or {})
            artifact_path = self._save_artifact(task_uuid, verification_payload.get("screenshotData"))
            if artifact_path:
                verification_payload["artifactPath"] = artifact_path
            self._update_task(
                task_uuid,
                status="needs_verify",
                message=verification_payload.get("message") or "发布任务需要人工验证，已终止自动执行",
                verification_data=verification_payload,
                artifact_path=artifact_path,
                finished=True,
            )
            task_logger.warning(
                "publish task needs manual verification task_uuid={} artifact_path={}",
                task_uuid,
                artifact_path,
            )
        except Exception as exc:
            self._update_task(
                task_uuid,
                status="failed",
                message=f"发布任务执行失败: {exc}",
                finished=True,
            )
            task_logger.exception("publish task failed task_uuid={} error={}", task_uuid, exc)

    def _execute_payload(self, payload):
        platform_type = int(payload["platformType"])
        title = payload["title"]
        file_path = self._resolve_payload_file_path(payload)
        account_file = Path(BASE_DIR / "cookiesFile" / payload["accountFilePath"])
        tags = payload.get("tags") or []
        publish_date = self._parse_publish_date(payload.get("publishDate"))

        if platform_type == 2:
            app = TencentVideo(
                title,
                file_path,
                tags,
                publish_date,
                account_file,
                payload.get("category") or TencentZoneTypes.LIFESTYLE.value,
                bool(payload.get("isDraft", False)),
            )
        elif platform_type == 3:
            thumbnail_path = self._resolve_thumbnail_path(payload)
            app = DouYinVideo(
                title,
                file_path,
                tags,
                publish_date,
                account_file,
                thumbnail_path,
                payload.get("productLink") or "",
                payload.get("productTitle") or "",
            )
        elif platform_type == 4:
            app = KSVideo(title, file_path, tags, publish_date, account_file)
        else:
            raise ValueError(f"当前不支持的平台类型: {platform_type}")

        asyncio.run(app.main(), debug=False)

    def _update_task(self, task_uuid, status, message, verification_data=None, artifact_path=None, finished=False):
        verification_json = json.dumps(verification_data, ensure_ascii=False) if verification_data else None
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                '''
                UPDATE publish_tasks
                SET status = ?,
                    message = ?,
                    verification_data = COALESCE(?, verification_data),
                    artifact_path = COALESCE(?, artifact_path),
                    finished_at = CASE WHEN ? THEN CURRENT_TIMESTAMP ELSE finished_at END,
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                ''',
                (status, message, verification_json, artifact_path, 1 if finished else 0, task_uuid),
            )
            conn.commit()
        self._sync_task(task_uuid)

    def _sync_task(self, task_uuid):
        if not self.sync_client or not self.sync_client.enabled:
            return

        task = self.get_task(task_uuid)
        if not task:
            return

        try:
            self.sync_client.sync_publish_task(task)
        except Exception as exc:
            log_throttled(
                task_logger,
                "WARNING",
                f"publish_task.sync_error:{task_uuid}",
                60,
                "publish task sync failed task_uuid={} error={}",
                task_uuid,
                exc,
            )
            return

    def _serialize_row(self, row):
        if row is None:
            return None
        payload = json.loads(row["payload_json"] or "{}")
        verification_data = json.loads(row["verification_data"]) if row["verification_data"] else None
        return {
            "id": row["id"],
            "taskUuid": row["task_uuid"],
            "source": row["source"],
            "platformType": row["platform_type"],
            "platformName": row["platform_name"],
            "accountName": row["account_name"],
            "accountFilePath": row["account_file_path"],
            "fileName": row["file_name"],
            "filePath": row["file_path"],
            "title": row["title"],
            "runAt": row["run_at"],
            "platformPublishAt": row["platform_publish_at"],
            "status": row["status"],
            "message": row["message"],
            "artifactPath": row["artifact_path"],
            "workerName": row["worker_name"],
            "createdAt": row["created_at"],
            "updatedAt": row["updated_at"],
            "startedAt": row["started_at"],
            "finishedAt": row["finished_at"],
            "verificationData": verification_data,
            "payload": payload,
        }

    def _save_artifact(self, task_uuid, screenshot_data):
        if not screenshot_data or "," not in screenshot_data:
            return None

        _, encoded = screenshot_data.split(",", 1)
        artifact_path = self._artifact_dir / f"{task_uuid}.png"
        with open(artifact_path, "wb") as artifact_file:
            artifact_file.write(base64.b64decode(encoded))
        return str(artifact_path.relative_to(BASE_DIR))

    def _get_account_lock(self, account_file_path):
        with self._account_locks_lock:
            lock = self._account_locks.get(account_file_path)
            if lock is None:
                lock = threading.Lock()
                self._account_locks[account_file_path] = lock
            return lock

    def _connect(self):
        return sqlite3.connect(self.db_path, timeout=30)

    @staticmethod
    def _normalize_datetime(value):
        if value in (None, "", 0, "0"):
            return None

        if isinstance(value, datetime):
            return value.strftime("%Y-%m-%d %H:%M:%S")

        value_str = str(value).strip().replace("T", " ")
        if not value_str:
            return None

        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%d %H:%M"):
            try:
                return datetime.strptime(value_str, fmt).strftime("%Y-%m-%d %H:%M:%S")
            except ValueError:
                continue
        return value_str

    @staticmethod
    def _parse_publish_date(value):
        if value in (None, "", 0, "0"):
            return 0

        value_str = str(value).strip().replace("T", " ")
        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%d %H:%M"):
            try:
                return datetime.strptime(value_str, fmt)
            except ValueError:
                continue
        raise ValueError(f"无法解析发布时间: {value}")

    @staticmethod
    def _display_file_name(file_path):
        file_name = Path(file_path).name
        if "_" in file_name:
            return file_name.split("_", 1)[1]
        return file_name

    def _normalize_file_inputs(self, data):
        file_inputs = []

        for file_path in [str(path).strip() for path in (data.get("fileList") or []) if str(path).strip()]:
            file_inputs.append(
                {
                    "mode": "video_record",
                    "filePath": file_path,
                    "displayPath": file_path,
                    "displayName": self._display_file_name(file_path),
                }
            )

        for item in data.get("fileItems") or []:
            if not isinstance(item, dict):
                raise ValueError("fileItems 中存在不支持的素材格式")
            resolved = resolve_material_reference(
                self.material_roots,
                root_name=item.get("root"),
                relative_path=item.get("path"),
                absolute_path=item.get("absolutePath"),
            )
            absolute_path = Path(resolved["absolutePath"])
            if not absolute_path.exists():
                raise ValueError(f"素材不存在: {absolute_path}")
            if not absolute_path.is_file():
                raise ValueError(f"素材不是文件: {absolute_path}")
            file_inputs.append(
                {
                    "mode": "material",
                    "root": resolved["rootName"],
                    "path": resolved["relativePath"],
                    "absolutePath": str(absolute_path),
                    "displayPath": f'{resolved["rootName"]}:{resolved["relativePath"]}',
                    "displayName": absolute_path.name,
                }
            )

        return file_inputs

    def _normalize_thumbnail_item(self, item):
        if not item:
            return None
        if not isinstance(item, dict):
            raise ValueError("thumbnailItem 格式不正确")
        resolved = resolve_material_reference(
            self.material_roots,
            root_name=item.get("root"),
            relative_path=item.get("path"),
            absolute_path=item.get("absolutePath"),
        )
        absolute_path = Path(resolved["absolutePath"])
        if not absolute_path.exists():
            raise ValueError(f"缩略图不存在: {absolute_path}")
        if not absolute_path.is_file():
            raise ValueError(f"缩略图不是文件: {absolute_path}")
        return {
            "root": resolved["rootName"],
            "path": resolved["relativePath"],
            "absolutePath": str(absolute_path),
        }

    def _resolve_payload_file_path(self, payload):
        file_source_mode = payload.get("fileSourceMode") or "video_record"
        if file_source_mode == "material":
            resolved = resolve_material_reference(
                self.material_roots,
                root_name=payload.get("materialRoot"),
                relative_path=payload.get("materialPath"),
                absolute_path=payload.get("sourceAbsolutePath"),
            )
            return resolved["absolutePath"]
        return str(Path(BASE_DIR / "videoFile" / payload["filePath"]))

    def _resolve_thumbnail_path(self, payload):
        thumbnail_mode = payload.get("thumbnailSourceMode") or "video_record"
        thumbnail_path = payload.get("thumbnailPath") or ""
        if not thumbnail_path and not payload.get("thumbnailAbsolutePath"):
            return ""
        if thumbnail_mode == "material":
            resolved = resolve_material_reference(
                self.material_roots,
                root_name=payload.get("thumbnailRoot"),
                relative_path=thumbnail_path,
                absolute_path=payload.get("thumbnailAbsolutePath"),
            )
            return resolved["absolutePath"]
        return str(Path(BASE_DIR / "videoFile" / thumbnail_path))

    @staticmethod
    def _is_future_datetime(value):
        if not value:
            return False
        try:
            return datetime.strptime(value, "%Y-%m-%d %H:%M:%S") > datetime.now()
        except ValueError:
            return True
