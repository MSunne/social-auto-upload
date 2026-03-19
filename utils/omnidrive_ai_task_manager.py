import json
import sqlite3
import threading
import uuid
from datetime import datetime
from pathlib import Path

from utils.log import ai_logger


FINAL_AI_TASK_STATUSES = {"success", "failed", "cancelled", "needs_verify"}


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
                    linked_publish_task_uuid TEXT,
                    artifact_refs_json TEXT,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    finished_at DATETIME
                )
                """
            )
            conn.commit()

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
                    payload_json, cloud_job_id, cloud_status, linked_publish_task_uuid, artifact_refs_json, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
        artifact_refs = data.get("artifactRefs") or []
        message = str(data.get("message") or "").strip() or "等待 OmniDrive 云端执行"

        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO omnidrive_ai_tasks (
                    task_uuid, source, job_type, model_name, skill_id, prompt, status, message,
                    payload_json, cloud_job_id, cloud_status, linked_publish_task_uuid, artifact_refs_json, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
                    linked_publish_task_uuid = COALESCE(excluded.linked_publish_task_uuid, omnidrive_ai_tasks.linked_publish_task_uuid),
                    artifact_refs_json = excluded.artifact_refs_json,
                    finished_at = excluded.finished_at,
                    updated_at = CURRENT_TIMESTAMP
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
                WHERE status IN ('queued_cloud', 'generating')
                ORDER BY updated_at ASC, id ASC
                LIMIT ?
                """,
                (max(1, min(int(limit), 500)),),
            )
            rows = cursor.fetchall()
        return [self._serialize_row(row) for row in rows]

    def update_cloud_binding(self, task_uuid, cloud_job_id, cloud_status, message=None):
        local_status = self._map_cloud_to_local_status(cloud_status, current_status="queued_cloud")
        finished_at = self._finished_at_for_status(local_status)
        with self._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_ai_tasks
                SET cloud_job_id = ?,
                    cloud_status = ?,
                    status = ?,
                    message = COALESCE(?, message),
                    finished_at = ?,
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                """,
                (cloud_job_id, cloud_status, local_status, message, finished_at, task_uuid),
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
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                """,
                (cloud_status, next_status, message, finished_at, task_uuid),
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
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                """,
                (
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
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                """,
                (next_status, message, finished_at, task_uuid),
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

    @staticmethod
    def _serialize_row(row):
        if row is None:
            return None
        item = dict(row)
        item["payload"] = json.loads(item.pop("payload_json") or "{}")
        item["artifactRefs"] = json.loads(item.pop("artifact_refs_json") or "[]")
        item["taskUuid"] = item.pop("task_uuid")
        item["jobType"] = item.pop("job_type")
        item["modelName"] = item.pop("model_name")
        item["skillId"] = item.pop("skill_id")
        item["cloudJobId"] = item.pop("cloud_job_id")
        item["cloudStatus"] = item.pop("cloud_status")
        item["linkedPublishTaskUuid"] = item.pop("linked_publish_task_uuid")
        item["createdAt"] = item.pop("created_at")
        item["updatedAt"] = item.pop("updated_at")
        item["finishedAt"] = item.pop("finished_at")
        return item

    @staticmethod
    def _map_cloud_to_local_status(cloud_status, current_status="queued_cloud"):
        cloud_status = str(cloud_status or "").strip()
        if cloud_status == "scheduled":
            return "scheduled"
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
