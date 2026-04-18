import json
import sqlite3
import threading
import uuid
from datetime import datetime
from pathlib import Path

from utils.log import ai_logger


FINAL_AI_TASK_STATUSES = {"success", "failed", "cancelled", "needs_verify"}
RECOVERED_PUBLISH_PENDING_MESSAGE = "OmniBull 重启后已恢复等待发布"
RECOVERED_PUBLISHING_MESSAGE = "OmniBull 重启后已恢复发布执行"


class OmniDriveAITaskManager:
    def __init__(self, db_path):
        self.db_path = Path(db_path)
        self._started = False
        self._lock = threading.Lock()

    def start(self):
        with self._lock:
            if self._started:
                return
            self.init_db()
            self._started = True
            ai_logger.info("omnidrive ai task manager started db_path={}", self.db_path)

    def init_db(self):
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS omnidrive_ai_tasks (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    task_uuid TEXT NOT NULL UNIQUE,
                    source TEXT NOT NULL DEFAULT 'local_ui',
                    job_type TEXT NOT NULL,
                    model_name TEXT NOT NULL,
                    skill_id TEXT,
                    prompt TEXT,
                    status TEXT NOT NULL DEFAULT 'queued_cloud',
                    message TEXT,
                    payload_json TEXT NOT NULL,
                    cloud_job_id TEXT,
                    cloud_status TEXT,
                    cloud_sync_dirty INTEGER NOT NULL DEFAULT 0,
                    last_cloud_sync_hash TEXT,
                    last_cloud_sync_at DATETIME,
                    linked_publish_task_uuid TEXT,
                    artifact_refs_json TEXT,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    finished_at DATETIME
                )
                """
            )
            self._ensure_column(cursor, "omnidrive_ai_tasks", "cloud_sync_dirty", "INTEGER NOT NULL DEFAULT 0")
            self._ensure_column(cursor, "omnidrive_ai_tasks", "last_cloud_sync_hash", "TEXT")
            self._ensure_column(cursor, "omnidrive_ai_tasks", "last_cloud_sync_at", "DATETIME")
            recovered = self._recover_interrupted_tasks(cursor)
            conn.commit()
        if recovered:
            ai_logger.info("omnidrive ai task startup recovery requeued_count={}", recovered)

    def create_task(self, data, source="local_ui"):
        job_type = str(data.get("jobType") or "").strip()
        model_name = str(data.get("modelName") or "").strip()
        prompt = str(data.get("prompt") or "").strip()
        if job_type not in {"image", "video", "chat"}:
            raise ValueError("jobType 仅支持 image、video 或 chat")
        if not model_name:
            raise ValueError("modelName 不能为空")
        if not prompt:
            raise ValueError("prompt 不能为空")

        payload = {
            "skillId": str(data.get("skillId") or "").strip() or None,
            "prompt": prompt,
            "inputPayload": data.get("inputPayload") or {},
            "publishPayload": data.get("publishPayload") or {},
            "runAt": data.get("runAt"),
            "jobType": job_type,
            "modelName": model_name,
        }
        source_category = str(data.get("sourceCategory") or "").strip() or self._default_source_category(source)
        execution_engine = str(data.get("executionEngine") or "").strip() or None
        correlation_id = str(data.get("correlationId") or "").strip() or None
        if source_category:
            payload["sourceCategory"] = source_category
        if execution_engine:
            payload["executionEngine"] = execution_engine
        if correlation_id:
            payload["correlationId"] = correlation_id
        task = {
            "taskUuid": str(data.get("taskUuid") or uuid.uuid4()),
            "source": str(source or "local_ui").strip() or "local_ui",
            "jobType": job_type,
            "modelName": model_name,
            "skillId": payload["skillId"],
            "prompt": prompt,
            "status": "queued_cloud",
            "message": "等待同步到 OmniDrive 云端",
            "payload": payload,
            "cloudJobId": None,
            "cloudStatus": "queued",
            "linkedPublishTaskUuid": None,
            "artifactRefs": [],
            "finishedAt": None,
        }

        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO omnidrive_ai_tasks (
                    task_uuid, source, job_type, model_name, skill_id, prompt, status, message,
                    payload_json, cloud_job_id, cloud_status, cloud_sync_dirty, last_cloud_sync_hash,
                    last_cloud_sync_at, linked_publish_task_uuid, artifact_refs_json, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    task["taskUuid"],
                    task["source"],
                    task["jobType"],
                    task["modelName"],
                    task["skillId"],
                    task["prompt"],
                    task["status"],
                    task["message"],
                    json.dumps(task["payload"], ensure_ascii=False),
                    task["cloudJobId"],
                    task["cloudStatus"],
                    1,
                    None,
                    None,
                    task["linkedPublishTaskUuid"],
                    json.dumps(task["artifactRefs"], ensure_ascii=False),
                    task["finishedAt"],
                ),
            )
            conn.commit()

        ai_logger.info(
            "ai task created task_uuid={} source={} job_type={} model_name={} skill_id={}",
            task["taskUuid"],
            task["source"],
            task["jobType"],
            task["modelName"],
            task["skillId"],
        )
        return self.get_task(task["taskUuid"])

    def import_remote_task(self, data):
        task_uuid = str(data.get("taskUuid") or uuid.uuid4()).strip()
        job_type = str(data.get("jobType") or "").strip()
        model_name = str(data.get("modelName") or "").strip()
        prompt = str(data.get("prompt") or "").strip()
        cloud_status = str(data.get("cloudStatus") or data.get("status") or "").strip() or "queued"
        local_status = str(data.get("status") or "").strip() or self._map_cloud_to_local_status(cloud_status, current_status="scheduled")
        payload = data.get("payload") or {}
        if isinstance(payload, dict):
            source_value = str(data.get("source") or "omnidrive_cloud").strip() or "omnidrive_cloud"
            payload.setdefault("sourceCategory", str(data.get("sourceCategory") or "").strip() or self._default_source_category(source_value))
            if str(data.get("executionEngine") or "").strip():
                payload.setdefault("executionEngine", str(data.get("executionEngine") or "").strip())
            if str(data.get("correlationId") or "").strip():
                payload.setdefault("correlationId", str(data.get("correlationId") or "").strip())
        artifact_refs = data.get("artifactRefs") or []
        message = str(data.get("message") or "").strip() or "等待 OmniDrive 云端执行"

        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO omnidrive_ai_tasks (
                    task_uuid, source, job_type, model_name, skill_id, prompt, status, message,
                    payload_json, cloud_job_id, cloud_status, cloud_sync_dirty, last_cloud_sync_hash,
                    last_cloud_sync_at, linked_publish_task_uuid, artifact_refs_json, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(task_uuid) DO UPDATE SET
                    source = excluded.source,
                    job_type = excluded.job_type,
                    model_name = excluded.model_name,
                    skill_id = excluded.skill_id,
                    prompt = excluded.prompt,
                    status = excluded.status,
                    message = excluded.message,
                    payload_json = excluded.payload_json,
                    cloud_job_id = excluded.cloud_job_id,
                    cloud_status = excluded.cloud_status,
                    cloud_sync_dirty = 0,
                    last_cloud_sync_hash = excluded.last_cloud_sync_hash,
                    last_cloud_sync_at = excluded.last_cloud_sync_at,
                    linked_publish_task_uuid = COALESCE(excluded.linked_publish_task_uuid, omnidrive_ai_tasks.linked_publish_task_uuid),
                    artifact_refs_json = excluded.artifact_refs_json,
                    finished_at = excluded.finished_at,
                    updated_at = CASE
                        WHEN omnidrive_ai_tasks.status IS NOT excluded.status
                          OR omnidrive_ai_tasks.cloud_status IS NOT excluded.cloud_status
                          OR omnidrive_ai_tasks.message IS NOT excluded.message
                          OR omnidrive_ai_tasks.artifact_refs_json IS NOT excluded.artifact_refs_json
                          OR omnidrive_ai_tasks.linked_publish_task_uuid IS NOT COALESCE(excluded.linked_publish_task_uuid, omnidrive_ai_tasks.linked_publish_task_uuid)
                        THEN CURRENT_TIMESTAMP
                        ELSE omnidrive_ai_tasks.updated_at
                    END
                """,
                (
                    task_uuid,
                    str(data.get("source") or "omnidrive_cloud").strip() or "omnidrive_cloud",
                    job_type,
                    model_name,
                    str(data.get("skillId") or "").strip() or None,
                    prompt,
                    local_status,
                    message,
                    json.dumps(payload, ensure_ascii=False),
                    str(data.get("cloudJobId") or "").strip() or None,
                    cloud_status,
                    0,
                    None,
                    None,
                    str(data.get("linkedPublishTaskUuid") or "").strip() or None,
                    json.dumps(artifact_refs, ensure_ascii=False),
                    self._finished_at_for_status(local_status),
                ),
            )
            conn.commit()
        ai_logger.debug(
            "ai remote task imported task_uuid={} cloud_job_id={} cloud_status={} local_status={}",
            task_uuid,
            data.get("cloudJobId"),
            cloud_status,
            local_status,
        )
        return self.get_task(task_uuid)

    def list_tasks(self, limit=100, status=None, source=None):
        limit = max(1, min(int(limit), 500))
        query = "SELECT * FROM omnidrive_ai_tasks"
        params = []
        conditions = []
        if status:
            conditions.append("status = ?")
            params.append(str(status).strip())
        if source:
            conditions.append("source = ?")
            params.append(str(source).strip())
        if conditions:
            query += " WHERE " + " AND ".join(conditions)
        query += " ORDER BY updated_at DESC, id DESC LIMIT ?"
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
                "SELECT * FROM omnidrive_ai_tasks WHERE task_uuid = ?",
                (task_uuid,),
            )
            row = cursor.fetchone()
        return self._serialize_row(row) if row else None

    def get_task_by_cloud_job(self, cloud_job_id):
        with self._connect() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                "SELECT * FROM omnidrive_ai_tasks WHERE cloud_job_id = ? ORDER BY updated_at DESC LIMIT 1",
                (cloud_job_id,),
            )
            row = cursor.fetchone()
        return self._serialize_row(row) if row else None

    def list_tasks_for_cloud_sync(self, limit=200):
        with self._connect() as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT * FROM omnidrive_ai_tasks
                WHERE cloud_sync_dirty = 1
                ORDER BY updated_at ASC, id ASC
                LIMIT ?
                """,
                (max(1, min(int(limit), 500)),),
            )
            rows = cursor.fetchall()
        return [self._serialize_row(row) for row in rows]

    def update_cloud_binding(
        self,
        task_uuid,
        cloud_job_id,
        cloud_status,
        message=None,
        *,
        source=None,
        job_type=None,
        model_name=None,
        skill_id=None,
        prompt=None,
        payload=None,
        linked_publish_task_uuid=None,
        cloud_sync_hash=None,
        clear_cloud_sync_dirty=False,
    ):
        current_task = self.get_task(task_uuid)
        current_status = str((current_task or {}).get("status") or "queued_cloud").strip() or "queued_cloud"
        local_status = self._map_cloud_to_local_status(cloud_status, current_status=current_status)
        finished_at = self._finished_at_for_status(local_status)
        reset_delivery_state = local_status in {"scheduled", "queued_cloud", "generating", "waiting_recharge"}
        payload_json = None
        if payload is not None:
            payload_json = json.dumps(payload, ensure_ascii=False)
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET source = COALESCE(?, source),
                    job_type = COALESCE(?, job_type),
                    model_name = COALESCE(?, model_name),
                    skill_id = COALESCE(?, skill_id),
                    prompt = COALESCE(?, prompt),
                    payload_json = COALESCE(?, payload_json),
                    cloud_job_id = ?,
                    cloud_status = ?,
                    cloud_sync_dirty = CASE
                        WHEN ? THEN 0
                        ELSE cloud_sync_dirty
                    END,
                    last_cloud_sync_hash = CASE
                        WHEN ? THEN ?
                        ELSE last_cloud_sync_hash
                    END,
                    last_cloud_sync_at = CASE
                        WHEN ? THEN CURRENT_TIMESTAMP
                        ELSE last_cloud_sync_at
                    END,
                    status = ?,
                    linked_publish_task_uuid = CASE
                        WHEN ? THEN NULL
                        ELSE COALESCE(?, linked_publish_task_uuid)
                    END,
                    artifact_refs_json = CASE
                        WHEN ? THEN '[]'
                        ELSE artifact_refs_json
                    END,
                    message = COALESCE(?, message),
                    finished_at = ?,
                    updated_at = CASE
                        WHEN status IS NOT ?
                          OR cloud_status IS NOT ?
                          OR cloud_sync_dirty IS NOT (CASE WHEN ? THEN 0 ELSE cloud_sync_dirty END)
                          OR last_cloud_sync_hash IS NOT (CASE WHEN ? THEN ? ELSE last_cloud_sync_hash END)
                          OR message IS NOT COALESCE(?, message)
                          OR cloud_job_id IS NOT ?
                          OR linked_publish_task_uuid IS NOT (
                              CASE
                                  WHEN ? THEN NULL
                                  ELSE COALESCE(?, linked_publish_task_uuid)
                              END
                          )
                        THEN CURRENT_TIMESTAMP
                        ELSE updated_at
                    END
                WHERE task_uuid = ?
                """,
                (
                    source,
                    job_type,
                    model_name,
                    skill_id,
                    prompt,
                    payload_json,
                    cloud_job_id,
                    cloud_status,
                    1 if clear_cloud_sync_dirty else 0,
                    1 if clear_cloud_sync_dirty else 0,
                    cloud_sync_hash,
                    1 if clear_cloud_sync_dirty else 0,
                    local_status,
                    1 if reset_delivery_state else 0,
                    linked_publish_task_uuid,
                    1 if reset_delivery_state else 0,
                    message,
                    finished_at,
                    local_status,
                    cloud_status,
                    1 if clear_cloud_sync_dirty else 0,
                    1 if clear_cloud_sync_dirty else 0,
                    cloud_sync_hash,
                    message,
                    cloud_job_id,
                    1 if reset_delivery_state else 0,
                    linked_publish_task_uuid,
                    task_uuid,
                ),
            )
            conn.commit()
        ai_logger.debug(
            "ai task cloud binding updated task_uuid={} cloud_job_id={} cloud_status={} local_status={}",
            task_uuid,
            cloud_job_id,
            cloud_status,
            local_status,
        )
        return self.get_task(task_uuid)

    def mark_cloud_state(self, task_uuid, cloud_status, message=None):
        task = self.get_task(task_uuid)
        if not task:
            return None
        current_status = str(task.get("status") or "queued_cloud")
        next_status = self._map_cloud_to_local_status(cloud_status, current_status=current_status)
        finished_at = self._finished_at_for_status(next_status)
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET cloud_status = ?,
                    status = ?,
                    message = COALESCE(?, message),
                    finished_at = ?,
                    updated_at = CASE
                        WHEN cloud_status IS NOT ?
                          OR status IS NOT ?
                          OR message IS NOT COALESCE(?, message)
                        THEN CURRENT_TIMESTAMP
                        ELSE updated_at
                    END
                WHERE task_uuid = ?
                """,
                (cloud_status, next_status, message, finished_at, cloud_status, next_status, message, task_uuid),
            )
            conn.commit()
        ai_logger.debug(
            "ai task cloud state updated task_uuid={} cloud_status={} local_status={}",
            task_uuid,
            cloud_status,
            next_status,
        )
        return self.get_task(task_uuid)

    def mark_result_imported(self, task_uuid, artifact_refs, linked_publish_task_uuid=None, message=None):
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET artifact_refs_json = ?,
                    linked_publish_task_uuid = COALESCE(?, linked_publish_task_uuid),
                    status = CASE
                        WHEN COALESCE(?, linked_publish_task_uuid) IS NOT NULL THEN 'publish_pending'
                        ELSE 'output_ready'
                    END,
                    message = COALESCE(?, message),
                    updated_at = CASE
                        WHEN artifact_refs_json IS NOT ?
                          OR linked_publish_task_uuid IS NOT COALESCE(?, linked_publish_task_uuid)
                          OR status IS NOT (CASE WHEN COALESCE(?, linked_publish_task_uuid) IS NOT NULL THEN 'publish_pending' ELSE 'output_ready' END)
                          OR message IS NOT COALESCE(?, message)
                        THEN CURRENT_TIMESTAMP
                        ELSE updated_at
                    END
                WHERE task_uuid = ?
                """,
                (
                    json.dumps(artifact_refs or [], ensure_ascii=False),
                    linked_publish_task_uuid,
                    linked_publish_task_uuid,
                    message,
                    json.dumps(artifact_refs or [], ensure_ascii=False),
                    linked_publish_task_uuid,
                    linked_publish_task_uuid,
                    message,
                    task_uuid,
                ),
            )
            conn.commit()
        ai_logger.info(
            "ai task result imported task_uuid={} artifact_count={} linked_publish_task_uuid={}",
            task_uuid,
            len(artifact_refs or []),
            linked_publish_task_uuid,
        )
        return self.get_task(task_uuid)

    def sync_linked_publish_status(self, task_uuid, publish_status, message=None):
        publish_status = str(publish_status or "").strip()
        if not publish_status:
            return self.get_task(task_uuid)
        status_map = {
            "pending": "publish_pending",
            "scheduled": "publish_pending",
            "running": "publishing",
            "success": "success",
            "needs_verify": "needs_verify",
            "failed": "failed",
            "cancelled": "cancelled",
        }
        next_status = status_map.get(publish_status, "publishing")
        finished_at = self._finished_at_for_status(next_status)
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET status = ?,
                    message = COALESCE(?, message),
                    finished_at = ?,
                    updated_at = CASE
                        WHEN status IS NOT ?
                          OR message IS NOT COALESCE(?, message)
                        THEN CURRENT_TIMESTAMP
                        ELSE updated_at
                    END
                WHERE task_uuid = ?
                """,
                (next_status, message, finished_at, next_status, message, task_uuid),
            )
            conn.commit()
        ai_logger.debug(
            "ai task linked publish status synced task_uuid={} publish_status={} local_status={}",
            task_uuid,
            publish_status,
            next_status,
        )
        return self.get_task(task_uuid)

    def summary(self):
        tasks = self.list_tasks(limit=500)
        by_status = {}
        by_source = {}
        for task in tasks:
            status = str(task.get("status") or "").strip()
            source = str(task.get("source") or "").strip() or "local_ui"
            by_status[status] = by_status.get(status, 0) + 1
            by_source[source] = by_source.get(source, 0) + 1
        return {
            "count": len(tasks),
            "byStatus": by_status,
            "bySource": by_source,
        }

    def _connect(self):
        return sqlite3.connect(self.db_path)

    def _recover_interrupted_tasks(self, cursor):
        recovered = 0
        try:
            cursor.execute(
                """
                SELECT task_uuid, status, message, linked_publish_task_uuid
                FROM omnidrive_ai_tasks
                WHERE status IN ('publish_pending', 'publishing')
                """
            )
            rows = cursor.fetchall()
        except sqlite3.OperationalError:
            return 0

        for row in rows:
            current_status = str(row[1] or "").strip()
            current_message = str(row[2] or "").strip()
            linked_publish_task_uuid = str(row[3] or "").strip()

            next_status = ""
            next_message = ""
            if linked_publish_task_uuid:
                try:
                    cursor.execute(
                        """
                        SELECT status, message
                        FROM publish_tasks
                        WHERE task_uuid = ?
                        LIMIT 1
                        """,
                        (linked_publish_task_uuid,),
                    )
                    publish_row = cursor.fetchone()
                except sqlite3.OperationalError:
                    publish_row = None

                if publish_row:
                    publish_status = str(publish_row[0] or "").strip()
                    publish_message = str(publish_row[1] or "").strip()
                    status_map = {
                        "pending": "publish_pending",
                        "scheduled": "publish_pending",
                        "running": "publishing",
                        "success": "success",
                        "needs_verify": "needs_verify",
                        "failed": "failed",
                        "cancelled": "cancelled",
                    }
                    next_status = status_map.get(publish_status, "")
                    if publish_status in {"pending", "scheduled"}:
                        next_message = publish_message or RECOVERED_PUBLISH_PENDING_MESSAGE
                    elif publish_status == "running":
                        next_message = publish_message or RECOVERED_PUBLISHING_MESSAGE
                    elif publish_status in {"success", "needs_verify", "failed", "cancelled"}:
                        next_message = publish_message

            if not next_status or (next_status == current_status and (not next_message or next_message == current_message)):
                continue

            finished_at = self._finished_at_for_status(next_status)
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET status = ?,
                    message = COALESCE(?, message),
                    finished_at = ?,
                    updated_at = CASE
                        WHEN status IS NOT ?
                          OR message IS NOT COALESCE(?, message)
                        THEN CURRENT_TIMESTAMP
                        ELSE updated_at
                    END
                WHERE task_uuid = ?
                """,
                (next_status, next_message, finished_at, next_status, next_message, str(row[0] or "").strip()),
            )
            recovered += cursor.rowcount

        return recovered

    @staticmethod
    def _serialize_row(row):
        if row is None:
            return None
        item = dict(row)
        payload = json.loads(item.pop("payload_json") or "{}")
        item["payload"] = payload
        item["artifactRefs"] = json.loads(item.pop("artifact_refs_json") or "[]")
        item["taskUuid"] = item.pop("task_uuid")
        item["jobType"] = item.pop("job_type")
        item["modelName"] = item.pop("model_name")
        item["skillId"] = item.pop("skill_id")
        item["cloudJobId"] = item.pop("cloud_job_id")
        item["cloudStatus"] = item.pop("cloud_status")
        item["cloudSyncDirty"] = bool(item.pop("cloud_sync_dirty"))
        item["lastCloudSyncHash"] = item.pop("last_cloud_sync_hash")
        item["lastCloudSyncAt"] = item.pop("last_cloud_sync_at")
        item["linkedPublishTaskUuid"] = item.pop("linked_publish_task_uuid")
        local_created_at = item.pop("created_at")
        local_updated_at = item.pop("updated_at")
        schedule_times = payload.get("scheduleTimes") if isinstance(payload, dict) else None
        if not isinstance(schedule_times, dict):
            schedule_times = None
        item["scheduleTimes"] = schedule_times
        item["localCreatedAt"] = local_created_at
        item["localUpdatedAt"] = local_updated_at
        item["createdAt"] = (
            str(schedule_times.get("createdAt") or "").strip() if schedule_times else ""
        ) or local_created_at
        item["updatedAt"] = (
            str(schedule_times.get("updatedAt") or "").strip() if schedule_times else ""
        ) or local_updated_at
        item["generateAt"] = (
            str(schedule_times.get("generateAt") or "").strip() if schedule_times else ""
        ) or str(payload.get("runAt") or "").strip() or None
        publish_payload = payload.get("publishPayload") if isinstance(payload, dict) else None
        item["sourceCategory"] = (
            str(payload.get("sourceCategory") or "").strip()
            if isinstance(payload, dict)
            else ""
        ) or OmniDriveAITaskManager._default_source_category(str(item.get("source") or "").strip())
        item["executionEngine"] = (
            str(payload.get("executionEngine") or "").strip()
            if isinstance(payload, dict)
            else ""
        ) or None
        item["correlationId"] = (
            str(payload.get("correlationId") or "").strip()
            if isinstance(payload, dict)
            else ""
        ) or None
        item["publishAt"] = (
            str(schedule_times.get("publishAt") or "").strip() if schedule_times else ""
        ) or str(payload.get("publishAt") or "").strip() or str(
            (publish_payload or {}).get("runAt") or (publish_payload or {}).get("requestedRun") or ""
        ).strip() or None
        item["finishedAt"] = item.pop("finished_at")
        return item

    @staticmethod
    def _default_source_category(source):
        normalized = str(source or "").strip()
        if normalized == "openclaw_skill":
            return "openclaw_direct"
        if normalized.startswith("hermes_"):
            return normalized
        if normalized == "local_ui":
            return "omnibull_local"
        return normalized or "omnibull_local"

    @staticmethod
    def _map_cloud_to_local_status(cloud_status, current_status="queued_cloud"):
        cloud_status = str(cloud_status or "").strip()
        if cloud_status == "scheduled":
            return "scheduled"
        if cloud_status == "waiting_recharge":
            return "waiting_recharge"
        if cloud_status in {"queued", "pending"}:
            return "queued_cloud"
        if cloud_status == "running":
            return "generating"
        if cloud_status in {"success", "completed"}:
            return current_status if current_status in {"output_ready", "publish_pending", "publishing", "success", "needs_verify"} else "output_ready"
        if cloud_status == "failed":
            return "failed"
        if cloud_status == "cancelled":
            return "cancelled"
        return current_status or "queued_cloud"

    @staticmethod
    def _finished_at_for_status(status):
        if status in FINAL_AI_TASK_STATUSES:
            return datetime.utcnow().strftime("%Y-%m-%d %H:%M:%S")
        return None

    @staticmethod
    def _ensure_column(cursor, table_name, column_name, definition):
        try:
            cursor.execute(f"ALTER TABLE {table_name} ADD COLUMN {column_name} {definition}")
        except sqlite3.OperationalError as exc:
            if "duplicate column name" not in str(exc).lower():
                raise
