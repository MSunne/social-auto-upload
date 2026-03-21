import json
import sqlite3
import shutil
import tempfile
import threading
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest import mock

import requests

from utils import omnidrive_agent as agent_module
from utils.omnidrive_ai_task_manager import OmniDriveAITaskManager
from utils.publish_task_manager import PublishTaskManager


class DummyPublishTaskManager:
    def __init__(self, worker_count=2):
        self.worker_count = worker_count
        self.enqueued_specs = []
        self.tasks = {}

    def list_tasks(self, limit=500, sources=None):
        items = list(self.tasks.values())
        if sources:
            allowed = set(sources)
            items = [item for item in items if item.get("source") in allowed]
        return items[:limit]

    def get_task(self, task_uuid):
        return self.tasks.get(task_uuid)

    def enqueue_specs(self, specs):
        for spec in specs:
            self.enqueued_specs.append(spec)
            self.tasks[spec["taskUuid"]] = spec

    def cancel_task_if_queued(self, task_uuid, message):
        task = self.tasks.get(task_uuid)
        if not task or task.get("status") not in {"pending", "scheduled"}:
            return False
        task["status"] = "cancelled"
        task["message"] = message
        return True


class DummyAITaskManager:
    def __init__(self):
        self.tasks = {}

    def summary(self):
        return {}

    def get_task(self, task_uuid):
        return self.tasks.get(task_uuid)

    def import_remote_task(self, data):
        task = {
            "taskUuid": data["taskUuid"],
            "source": data.get("source"),
            "jobType": data.get("jobType"),
            "modelName": data.get("modelName"),
            "skillId": data.get("skillId"),
            "prompt": data.get("prompt"),
            "status": data.get("status"),
            "message": data.get("message"),
            "payload": data.get("payload") or {},
            "cloudJobId": data.get("cloudJobId"),
            "cloudStatus": data.get("cloudStatus"),
            "linkedPublishTaskUuid": data.get("linkedPublishTaskUuid"),
            "artifactRefs": data.get("artifactRefs") or [],
        }
        self.tasks[task["taskUuid"]] = task
        return task

    def update_cloud_binding(self, task_uuid, cloud_job_id, cloud_status, message=None):
        task = self.tasks[task_uuid]
        task["cloudJobId"] = cloud_job_id
        task["cloudStatus"] = cloud_status
        if message:
            task["message"] = message
        return task

    def mark_cloud_state(self, task_uuid, cloud_status, message=None):
        task = self.tasks[task_uuid]
        task["cloudStatus"] = cloud_status
        if message:
            task["message"] = message
        return task

    def mark_result_imported(self, task_uuid, artifact_refs, linked_publish_task_uuid=None, message=None):
        task = self.tasks[task_uuid]
        task["artifactRefs"] = artifact_refs
        if linked_publish_task_uuid:
            task["linkedPublishTaskUuid"] = linked_publish_task_uuid
            task["status"] = "publish_pending"
        else:
            task["status"] = "output_ready"
        if message:
            task["message"] = message
        return task

    def list_tasks(self, limit=100, status=None, source=None):
        items = list(self.tasks.values())
        if status:
            items = [item for item in items if item.get("status") == status]
        if source:
            items = [item for item in items if item.get("source") == source]
        return items[:limit]

    def list_tasks_for_cloud_sync(self, limit=200):
        return list(self.tasks.values())[:limit]

    def sync_linked_publish_status(self, task_uuid, publish_status, message=None):
        task = self.tasks[task_uuid]
        task["status"] = publish_status
        if message:
            task["message"] = message
        return task


class OmniDriveBridgeTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = Path(tempfile.mkdtemp(prefix="omnidrive-agent-test-"))
        self.addCleanup(lambda: shutil.rmtree(self.temp_dir, ignore_errors=True))

    @staticmethod
    def ensure_user_info_table(db_path):
        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS user_info (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    type INTEGER NOT NULL,
                    filePath TEXT NOT NULL,
                    userName TEXT NOT NULL,
                    status INTEGER DEFAULT 0
                )
                """
            )
            conn.commit()

    def make_bridge(self, publish_task_manager=None, ai_task_manager=None):
        publish_task_manager = publish_task_manager or DummyPublishTaskManager()
        ai_task_manager = ai_task_manager or DummyAITaskManager()
        with mock.patch.object(agent_module, "BASE_DIR", self.temp_dir):
            bridge = agent_module.OmniDriveBridge(
                db_path=self.temp_dir / "database.db",
                cloud_base_url="https://cloud.test",
                agent_key="agent-key",
                run_login_fn=lambda *args, **kwargs: None,
                publish_task_manager=publish_task_manager,
                ai_task_manager=ai_task_manager,
                material_roots={},
                device_name="test-device",
                device_code="device-1",
                generated_root_name="generated",
                generated_root_path=self.temp_dir / "generated",
                poll_interval=5,
                heartbeat_interval=30,
                account_sync_interval=60,
                material_sync_interval=300,
                skill_sync_interval=120,
                publish_sync_interval=5,
            )
        return bridge

    def test_sync_skills_cleans_stale_assets_and_records_local_paths(self):
        bridge = self.make_bridge()
        skill_assets_dir = bridge._skill_cache_dir / "skill-1" / "assets"
        skill_assets_dir.mkdir(parents=True, exist_ok=True)
        stale_asset = skill_assets_dir / "stale.txt"
        stale_metadata = skill_assets_dir / "stale.txt.json"
        stale_asset.write_text("old", encoding="utf-8")
        stale_metadata.write_text("{}", encoding="utf-8")

        cloud_payload = {
            "items": [
                {
                    "revision": "rev-2",
                    "skill": {
                        "id": "skill-1",
                        "name": "Knowledge Skill",
                        "description": "latest knowledge",
                        "outputType": "text",
                        "isEnabled": True,
                    },
                    "assets": [
                        {
                            "id": "asset-1",
                            "fileName": "guide.md",
                            "assetType": "knowledge",
                            "mimeType": "text/markdown",
                            "publicUrl": "/objects/guide.md",
                        }
                    ],
                    "sync": {
                        "syncStatus": "failed",
                        "syncedRevision": "rev-1",
                    },
                }
            ],
            "retiredItems": [],
        }
        sync_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/skills/device-1":
                return cloud_payload
            if method == "POST" and path == "/api/v1/agent/skills/sync":
                sync_calls.append(payload)
                return {"ok": True}
            raise AssertionError(f"unexpected request {method} {path}")

        response = mock.Mock()
        response.content = b"# Guide"
        response.raise_for_status.return_value = None

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            with mock.patch.object(bridge._session, "get", return_value=response) as mock_get:
                bridge._sync_skills()

        mock_get.assert_called_once_with("https://cloud.test/objects/guide.md", timeout=bridge.http_timeout)
        self.assertFalse(stale_asset.exists())
        self.assertFalse(stale_metadata.exists())

        manifest_path = bridge._skill_cache_dir / "skill-1" / "manifest.json"
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        asset = manifest["assets"][0]
        self.assertEqual(asset["downloadStatus"], "success")
        self.assertTrue(Path(asset["localPath"]).exists())
        self.assertTrue(Path(asset["metadataPath"]).exists())

        cached_skills = bridge.list_cached_skills(include_assets=True)
        self.assertEqual(len(cached_skills), 1)
        self.assertEqual(cached_skills[0]["skillId"], "skill-1")
        self.assertEqual(cached_skills[0]["assetCount"], 1)
        self.assertEqual(cached_skills[0]["assets"][0]["downloadStatus"], "success")
        self.assertEqual(sync_calls[0]["items"][0]["syncStatus"], "success")

    def test_import_remote_publish_tasks_continues_after_claim_failure(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        bridge = self.make_bridge(publish_task_manager=publish_task_manager)
        sync_payloads = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/publish-tasks/device-1":
                return [{"id": "bad-task"}, {"id": "good-task"}]
            if method == "GET" and path == "/api/v1/agent/publish-tasks/bad-task/package":
                return {"task": {"id": "bad-task"}}
            if method == "POST" and path == "/api/v1/agent/publish-tasks/bad-task/claim":
                raise make_http_error(409, {"error": "Publish task is not claimable"})
            if method == "GET" and path == "/api/v1/agent/publish-tasks/good-task/package":
                return {"task": {"id": "good-task"}}
            if method == "POST" and path == "/api/v1/agent/publish-tasks/good-task/claim":
                return {"leaseToken": "lease-good", "leaseExpiresAt": "2026-03-18T00:00:00Z"}
            if method == "POST" and path == "/api/v1/agent/publish-tasks/sync":
                sync_payloads.append(payload)
                return {"ok": True}
            raise AssertionError(f"unexpected request {method} {path}")

        good_spec = {
            "taskUuid": "good-task",
            "source": "omnidrive_agent",
            "platformName": "抖音",
            "accountName": "demo-account",
            "title": "demo-title",
            "payload": {},
        }

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            with mock.patch.object(bridge, "_build_local_task_spec", return_value=good_spec):
                bridge._import_remote_publish_tasks()

        self.assertEqual(len(publish_task_manager.enqueued_specs), 1)
        self.assertEqual(publish_task_manager.enqueued_specs[0]["taskUuid"], "good-task")
        self.assertEqual(len(sync_payloads), 1)
        self.assertEqual(sync_payloads[0]["id"], "good-task")
        self.assertIn("bad-task", bridge.status().get("lastError") or "")

    def test_sync_local_publish_tasks_stops_retrying_final_task_after_device_conflict(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        publish_task_manager.tasks["local-bridge-smoke-task"] = {
            "taskUuid": "local-bridge-smoke-task",
            "source": "openclaw_skill",
            "platformName": "抖音",
            "accountName": "SmokeAccount",
            "title": "Local Bridge Smoke Task",
            "status": "cancelled",
            "message": "smoke cleanup",
            "updatedAt": "2026-03-16 03:20:44",
            "payload": {
                "tags": ["smoke"],
                "publishDate": 0,
                "isDraft": False,
                "productLink": "",
                "productTitle": "",
                "omnidriveMaterialRefs": [{"root": "testRoot", "path": "sample.txt", "role": "media"}],
            },
        }
        bridge = self.make_bridge(publish_task_manager=publish_task_manager)
        request_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            request_calls.append((method, path, payload))
            if method == "POST" and path == "/api/v1/agent/publish-tasks/sync":
                raise make_http_error(409, {"error": "Publish task belongs to a different device"})
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._sync_local_publish_tasks()
            bridge._sync_local_publish_tasks()

        self.assertEqual(len(request_calls), 1)
        lease = bridge._get_lease("local-bridge-smoke-task")
        self.assertIsNotNone(lease)
        self.assertEqual(lease["last_synced_status"], "cancelled")
        self.assertEqual(lease["last_synced_updated_at"], "2026-03-16 03:20:44")

    def test_sync_local_publish_tasks_stops_retrying_final_task_after_status_transition_conflict(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        publish_task_manager.tasks["local-bridge-transition-task"] = {
            "taskUuid": "local-bridge-transition-task",
            "source": "local_api",
            "platformName": "抖音",
            "accountName": "RetryAccount",
            "title": "Retry Publish Task",
            "status": "needs_verify",
            "message": "requires manual verification",
            "updatedAt": "2026-03-21 16:06:51",
            "payload": {
                "tags": ["verify"],
                "publishDate": 0,
                "isDraft": False,
                "productLink": "",
                "productTitle": "",
            },
        }
        bridge = self.make_bridge(publish_task_manager=publish_task_manager)
        request_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            request_calls.append((method, path, payload))
            if method == "POST" and path == "/api/v1/agent/publish-tasks/sync":
                raise make_http_error(409, {"error": "Publish task status transition is not allowed"})
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._sync_local_publish_tasks()
            bridge._sync_local_publish_tasks()

        self.assertEqual(len(request_calls), 1)
        lease = bridge._get_lease("local-bridge-transition-task")
        self.assertIsNotNone(lease)
        self.assertEqual(lease["last_synced_status"], "needs_verify")
        self.assertEqual(lease["last_synced_updated_at"], "2026-03-21 16:06:51")

    def test_resolve_ai_publish_target_falls_back_to_local_account_when_cookie_path_is_missing(self):
        bridge = self.make_bridge()
        self.ensure_user_info_table(bridge.db_path)
        with sqlite3.connect(bridge.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (4, "ks-real.json", "测试快手_乔总", 1),
            )
            conn.commit()

        resolved = bridge._resolve_ai_publish_target(
            platform_value="快手",
            account_name="测试快手_乔总",
            account_file_path="missing-mock.json",
        )

        self.assertEqual(
            resolved,
            {
                "platformType": 4,
                "platformName": "快手",
                "accountName": "测试快手_乔总",
                "accountFilePath": "ks-real.json",
            },
        )

    def test_import_remote_account_skill_job_falls_back_to_cloud_job_id_and_creates_publish_tasks(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        ai_task_manager = DummyAITaskManager()
        bridge = self.make_bridge(
            publish_task_manager=publish_task_manager,
            ai_task_manager=ai_task_manager,
        )
        delivery_updates = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1":
                return [
                    {
                        "job": {
                            "id": "cloud-job-1",
                            "status": "success",
                            "source": "account_skill_binding",
                            "jobType": "video",
                            "modelName": "veo",
                            "prompt": "生成春季广告视频",
                            "inputPayload": {
                                "publishPayload": {
                                    "title": "春季广告",
                                    "contentText": "新品上新",
                                    "targets": [
                                        {"platform": "抖音", "accountName": "账号A"},
                                        {"platform": "快手", "accountName": "账号B"},
                                    ],
                                }
                            },
                        },
                        "artifacts": [{"artifactKey": "video-1", "artifactType": "video"}],
                    }
                ]
            if method == "POST" and path == "/api/v1/agent/ai-jobs/cloud-job-1/delivery":
                delivery_updates.append(payload)
                return {"ok": True}
            raise AssertionError(f"unexpected request {method} {path}")

        artifact_refs = [
            {
                "root": "generated",
                "path": "cloud-job-1/video.mp4",
                "absolutePath": str(self.temp_dir / "generated" / "cloud-job-1" / "video.mp4"),
                "name": "video.mp4",
                "role": "media",
            }
        ]

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            with mock.patch.object(bridge, "_download_ai_artifacts", return_value=artifact_refs):
                with mock.patch.object(bridge, "_sync_materials"):
                    with mock.patch.object(
                        bridge,
                        "_load_local_account_by_name",
                        side_effect=lambda platform, account: {
                            "filePath": f"/tmp/{platform}-{account}.json",
                            "userName": account,
                        },
                    ):
                        imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 1)
        self.assertEqual(len(publish_task_manager.enqueued_specs), 2)
        self.assertEqual(
            {spec["platformName"] for spec in publish_task_manager.enqueued_specs},
            {"抖音", "快手"},
        )
        self.assertEqual(
            {spec["accountName"] for spec in publish_task_manager.enqueued_specs},
            {"账号A", "账号B"},
        )
        self.assertEqual(
            publish_task_manager.enqueued_specs[0]["payload"]["omnidriveAICloudJobId"],
            "cloud-job-1",
        )
        self.assertEqual(
            ai_task_manager.get_task("cloud-job-1")["linkedPublishTaskUuid"],
            publish_task_manager.enqueued_specs[0]["taskUuid"],
        )
        self.assertEqual(delivery_updates[0]["status"], "publish_queued")
        self.assertEqual(
            delivery_updates[0]["localPublishTaskId"],
            publish_task_manager.enqueued_specs[0]["taskUuid"],
        )

    def test_import_remote_ai_job_syncs_generated_materials_before_enqueue(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        ai_task_manager = DummyAITaskManager()
        bridge = self.make_bridge(
            publish_task_manager=publish_task_manager,
            ai_task_manager=ai_task_manager,
        )
        ai_task_manager.tasks["local-ai-1"] = {
            "taskUuid": "local-ai-1",
            "source": "omnibull_local",
            "jobType": "video",
            "modelName": "veo",
            "prompt": "生成春季广告视频",
            "status": "output_ready",
            "message": "ready",
            "payload": {
                "publishPayload": {
                    "title": "春季广告",
                    "contentText": "新品上新",
                    "platform": "快手",
                    "accountName": "账号A",
                }
            },
            "cloudJobId": "cloud-job-1",
        }
        delivery_updates = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1":
                return [
                    {
                        "job": {
                            "id": "cloud-job-1",
                            "localTaskId": "local-ai-1",
                            "status": "success",
                            "message": "done",
                        },
                        "artifacts": [{"artifactKey": "video-1", "artifactType": "video"}],
                    }
                ]
            if method == "POST" and path == "/api/v1/agent/ai-jobs/cloud-job-1/delivery":
                delivery_updates.append(payload)
                return {"ok": True}
            raise AssertionError(f"unexpected request {method} {path}")

        artifact_refs = [
            {
                "root": bridge.generated_root_name,
                "path": "local-ai-1/video.mp4",
                "absolutePath": str(self.temp_dir / "generated" / "local-ai-1" / "video.mp4"),
                "name": "video.mp4",
                "role": "media",
            }
        ]

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            with mock.patch.object(bridge, "_download_ai_artifacts", return_value=artifact_refs):
                with mock.patch.object(bridge, "_sync_materials") as sync_materials:
                    with mock.patch.object(bridge, "_enqueue_publish_from_ai_task", return_value="publish-1"):
                        imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 1)
        sync_materials.assert_called_once()
        self.assertEqual(delivery_updates[0]["status"], "publish_queued")

    def test_sync_local_ai_tasks_skips_non_syncable_sources(self):
        ai_task_manager = DummyAITaskManager()
        ai_task_manager.tasks["remote-ai-1"] = {
            "taskUuid": "remote-ai-1",
            "source": "omnidrive_cloud",
            "jobType": "video",
            "modelName": "veo",
            "prompt": "remote",
            "status": "queued_cloud",
            "message": "from cloud",
            "payload": {"inputPayload": {}, "publishPayload": {}},
        }
        ai_task_manager.tasks["local-ai-1"] = {
            "taskUuid": "local-ai-1",
            "source": "local_ui",
            "jobType": "video",
            "modelName": "veo",
            "prompt": "local",
            "status": "queued_cloud",
            "message": "from local",
            "payload": {"inputPayload": {}, "publishPayload": {}},
        }
        bridge = self.make_bridge(ai_task_manager=ai_task_manager)
        sync_payloads = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "POST" and path == "/api/v1/agent/ai-jobs/sync":
                sync_payloads.append(payload)
                return {"job": {"id": "cloud-local-1", "status": "queued", "message": "ok"}}
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            mirrored = bridge._sync_local_ai_tasks()

        self.assertEqual(mirrored, 1)
        self.assertEqual(len(sync_payloads), 1)
        self.assertEqual(sync_payloads[0]["id"], "local-ai-1")

    def test_bridge_datetime_helpers_preserve_cloud_schedule_in_local_time(self):
        remote_run_at = "2026-03-20T13:34:45Z"
        expected_local = (
            datetime.fromisoformat("2026-03-20T13:34:45+00:00")
            .astimezone()
            .strftime("%Y-%m-%d %H:%M:%S")
        )
        expected_rfc3339 = (
            datetime.strptime(expected_local, "%Y-%m-%d %H:%M:%S")
            .replace(tzinfo=datetime.now().astimezone().tzinfo or timezone.utc)
            .astimezone(timezone.utc)
            .isoformat()
            .replace("+00:00", "Z")
        )

        self.assertEqual(agent_module.OmniDriveBridge._normalize_datetime(remote_run_at), expected_local)
        self.assertEqual(agent_module.OmniDriveBridge._to_rfc3339(expected_local), expected_rfc3339)


class PublishTaskManagerDatetimeTests(unittest.TestCase):
    def test_publish_task_manager_keeps_timezone_aware_publish_times_local(self):
        remote_run_at = "2026-03-20T13:34:45Z"
        expected_local = (
            datetime.fromisoformat("2026-03-20T13:34:45+00:00")
            .astimezone()
            .strftime("%Y-%m-%d %H:%M:%S")
        )

        normalized = PublishTaskManager._normalize_datetime(remote_run_at)
        parsed = PublishTaskManager._parse_publish_date(remote_run_at)

        self.assertEqual(normalized, expected_local)
        self.assertEqual(parsed.strftime("%Y-%m-%d %H:%M:%S"), expected_local)
        self.assertIsNone(parsed.tzinfo)

    def test_publish_task_manager_repairs_future_omnidrive_ai_tasks_after_restart(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-test-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        ai_manager = OmniDriveAITaskManager(db_path)
        ai_manager.init_db()
        future_publish_at = datetime.now(timezone.utc).replace(microsecond=0) + timedelta(hours=1)
        ai_manager.import_remote_task(
            {
                "taskUuid": "local-ai-1",
                "jobType": "video",
                "modelName": "veo",
                "prompt": "future publish",
                "status": "publish_pending",
                "cloudStatus": "publish_pending",
                "payload": {
                    "publishAt": future_publish_at.isoformat().replace("+00:00", "Z"),
                    "publishPayload": {
                        "runAt": future_publish_at.isoformat().replace("+00:00", "Z"),
                        "requestedRun": future_publish_at.isoformat().replace("+00:00", "Z"),
                    },
                },
            }
        )

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        manager.init_db()

        broken_local_run_at = future_publish_at.strftime("%Y-%m-%d %H:%M:%S")
        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "publish-1",
                    "omnidrive_ai",
                    3,
                    "抖音",
                    "测试账号",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:local-ai-1/video.mp4",
                    "future publish",
                    broken_local_run_at,
                    broken_local_run_at,
                    "failed",
                    "OmniBull 重启导致任务中断，请按需重试",
                    json.dumps({"omnidriveAITaskUuid": "local-ai-1"}, ensure_ascii=False),
                ),
            )
            conn.commit()

        manager.init_db()
        repaired_task = manager.get_task("publish-1")
        expected_local_run_at = future_publish_at.astimezone().strftime("%Y-%m-%d %H:%M:%S")

        self.assertEqual(repaired_task["status"], "scheduled")
        self.assertEqual(repaired_task["runAt"], expected_local_run_at)
        self.assertEqual(repaired_task["platformPublishAt"], expected_local_run_at)
        self.assertEqual(repaired_task["message"], "等待 AI 产物定时发布")
        self.assertIsNone(repaired_task["startedAt"])
        self.assertIsNone(repaired_task["finishedAt"])

    def test_publish_task_manager_requeues_running_tasks_after_restart(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-recover-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        manager.init_db()

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json,
                    worker_name, started_at, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
                """,
                (
                    "publish-running-1",
                    "omnidrive_ai",
                    4,
                    "快手",
                    "测试快手_乔总",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "restart recover",
                    None,
                    None,
                    "running",
                    "任务执行中",
                    json.dumps({}, ensure_ascii=False),
                    "worker-1",
                ),
            )
            conn.commit()

        manager.init_db()
        recovered_task = manager.get_task("publish-running-1")

        self.assertEqual(recovered_task["status"], "pending")
        self.assertEqual(recovered_task["message"], "OmniBull 重启后已恢复待执行")
        self.assertIsNone(recovered_task["workerName"])
        self.assertIsNone(recovered_task["startedAt"])
        self.assertIsNone(recovered_task["finishedAt"])

    def test_publish_task_manager_keeps_historical_failed_tasks_stopped_after_restart(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-no-replay-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        manager.init_db()

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json,
                    worker_name, started_at, finished_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
                """,
                (
                    "publish-failed-history-1",
                    "local_api",
                    3,
                    "抖音",
                    "历史任务账号",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:history/video.mp4",
                    "historical failed task",
                    None,
                    None,
                    "failed",
                    "OmniBull 重启导致任务中断，请按需重试",
                    json.dumps({}, ensure_ascii=False),
                    "worker-2",
                ),
            )
            conn.commit()

        manager.init_db()
        historical_task = manager.get_task("publish-failed-history-1")

        self.assertEqual(historical_task["status"], "failed")
        self.assertEqual(historical_task["message"], "OmniBull 重启导致任务中断，请按需重试")
        self.assertEqual(historical_task["workerName"], "worker-2")
        self.assertIsNotNone(historical_task["startedAt"])
        self.assertIsNotNone(historical_task["finishedAt"])

class OmniDriveAITaskManagerRecoveryTests(unittest.TestCase):
    def test_ai_task_manager_recovers_inflight_publish_state_after_restart(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="ai-task-manager-recover-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        publish_manager = PublishTaskManager(db_path=db_path, material_roots={})
        publish_manager.init_db()

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "publish-ai-1",
                    "omnidrive_ai",
                    4,
                    "快手",
                    "测试快手_乔总",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "recover linked ai task",
                    None,
                    None,
                    "running",
                    "任务执行中",
                    json.dumps({}, ensure_ascii=False),
                ),
            )
            conn.commit()

        publish_manager.init_db()
        ai_manager = OmniDriveAITaskManager(db_path)
        ai_manager.init_db()

        with sqlite3.connect(db_path) as conn:
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
                    "local-ai-1",
                    "omnibull_local",
                    "video",
                    "veo-3.1-fast-fl",
                    None,
                    "生成玩具视频",
                    "publishing",
                    "发布执行中",
                    json.dumps({}, ensure_ascii=False),
                    "cloud-ai-1",
                    "success",
                    "publish-ai-1",
                    json.dumps([], ensure_ascii=False),
                    None,
                ),
            )
            conn.commit()

        ai_manager.init_db()
        recovered_task = ai_manager.get_task("local-ai-1")

        self.assertEqual(recovered_task["status"], "publish_pending")
        self.assertEqual(recovered_task["message"], "OmniBull 重启后已恢复待执行")
        self.assertIsNone(recovered_task["finishedAt"])

    def test_publish_task_manager_worker_loop_accepts_serialized_account_file_path(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-worker-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = PublishTaskManager(db_path=db_path, material_roots={})

        claimed_task = {
            "taskUuid": "publish-worker-1",
            "accountFilePath": "cookies/demo.json",
        }
        claim_count = {"value": 0}

        def fake_claim(_worker_name):
            if claim_count["value"] == 0:
                claim_count["value"] += 1
                return claimed_task
            manager._stop_event.set()
            return None

        with mock.patch.object(manager, "_claim_next_ready_task", side_effect=fake_claim):
            with mock.patch.object(manager, "_run_task") as run_task:
                with mock.patch("utils.publish_task_manager.time.sleep", return_value=None):
                    manager._worker_loop("worker-1")

        run_task.assert_called_once_with(claimed_task)

    def test_publish_task_manager_worker_loop_keeps_running_after_worker_error(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-worker-recover-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = PublishTaskManager(db_path=db_path, material_roots={})
        lock = threading.Lock()

        first_task = {
            "taskUuid": "publish-worker-err",
            "accountFilePath": "cookies/one.json",
        }
        second_task = {
            "taskUuid": "publish-worker-ok",
            "accountFilePath": "cookies/two.json",
        }
        claims = iter([first_task, second_task, None])

        def fake_claim(_worker_name):
            task = next(claims)
            if task is None:
                manager._stop_event.set()
            return task

        run_calls = []

        def fake_run(task):
            run_calls.append(task["taskUuid"])
            if task["taskUuid"] == "publish-worker-err":
                raise RuntimeError("boom")

        with mock.patch.object(manager, "_claim_next_ready_task", side_effect=fake_claim):
            with mock.patch.object(manager, "_get_account_lock", return_value=lock):
                with mock.patch.object(manager, "_run_task", side_effect=fake_run):
                    with mock.patch("utils.publish_task_manager.time.sleep", return_value=None):
                        manager._worker_loop("worker-1")

        self.assertEqual(run_calls, ["publish-worker-err", "publish-worker-ok"])

    def test_publish_task_manager_rejects_account_platform_mismatch(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-account-mismatch-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = PublishTaskManager(db_path=db_path, material_roots={})

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                CREATE TABLE user_info (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    type INTEGER NOT NULL,
                    filePath TEXT NOT NULL,
                    userName TEXT NOT NULL,
                    status INTEGER DEFAULT 0
                )
                """
            )
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (4, "kuaishou-account.json", "测试快手_乔总", 1),
            )
            conn.commit()

        with self.assertRaisesRegex(ValueError, "账号与平台不匹配"):
            manager._build_task_specs(
                {
                    "type": 3,
                    "title": "错误的平台组合",
                    "tags": [],
                    "accountList": ["kuaishou-account.json"],
                    "fileList": ["demo/video.mp4"],
                },
                source="omnidrive_ai",
            )


def make_http_error(status_code, payload):
    error = requests.HTTPError(f"http {status_code}")
    response = mock.Mock()
    response.status_code = status_code
    response.json.return_value = payload
    response.text = json.dumps(payload, ensure_ascii=False)
    error.response = response
    return error


if __name__ == "__main__":
    unittest.main()
