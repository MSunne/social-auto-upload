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

    def realign_omnidrive_ai_task(self, task_uuid, intended_run_at, intended_publish_at=None):
        task = self.tasks.get(task_uuid)
        if not task:
            return False
        normalized_run_at = PublishTaskManager._normalize_datetime(intended_run_at)
        normalized_publish_at = PublishTaskManager._normalize_datetime(intended_publish_at) or normalized_run_at
        task["runAt"] = normalized_run_at
        task["platformPublishAt"] = normalized_publish_at
        is_future = agent_module.OmniDriveBridge._is_future_datetime(normalized_run_at)
        task["status"] = "scheduled" if is_future else "pending"
        task["message"] = "等待 AI 产物定时发布" if is_future else "等待 AI 产物发布"
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

    def update_cloud_binding(self, task_uuid, cloud_job_id, cloud_status, message=None, **kwargs):
        task = self.tasks[task_uuid]
        task["cloudJobId"] = cloud_job_id
        task["cloudStatus"] = cloud_status
        if "source" in kwargs and kwargs["source"] is not None:
            task["source"] = kwargs["source"]
        if "job_type" in kwargs and kwargs["job_type"] is not None:
            task["jobType"] = kwargs["job_type"]
        if "model_name" in kwargs and kwargs["model_name"] is not None:
            task["modelName"] = kwargs["model_name"]
        if "skill_id" in kwargs:
            task["skillId"] = kwargs["skill_id"]
        if "prompt" in kwargs and kwargs["prompt"] is not None:
            task["prompt"] = kwargs["prompt"]
        if "payload" in kwargs and kwargs["payload"] is not None:
            task["payload"] = kwargs["payload"]
        if "linked_publish_task_uuid" in kwargs and kwargs["linked_publish_task_uuid"] is not None:
            task["linkedPublishTaskUuid"] = kwargs["linked_publish_task_uuid"]
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
                device_fingerprint="fingerprint-1",
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

    def test_heartbeat_includes_device_fingerprint(self):
        bridge = self.make_bridge()
        bridge._update_state(
            running=True,
            cloudReachable=False,
            cloudRetryAt="2026-04-06T10:46:21+08:00",
            lastError="404 Client Error: Not Found",
            lastLoginPollAt="2026-04-06T10:46:16+08:00",
        )
        request_calls = []

        def fake_request(method, path, *, params=None, payload=None, track_bridge_health=True):
            request_calls.append((method, path, payload))
            return {"device": {"deviceCode": bridge.device_code}}

        with mock.patch.object(agent_module, "get_local_ip", return_value="192.168.1.10"), mock.patch.object(
            bridge, "_request", side_effect=fake_request
        ):
            bridge._heartbeat()

        self.assertEqual(len(request_calls), 1)
        _, path, payload = request_calls[0]
        self.assertEqual(path, "/api/v1/agent/heartbeat")
        self.assertEqual(payload["deviceFingerprint"], "fingerprint-1")
        self.assertEqual(payload["runtimePayload"]["deviceFingerprint"], "fingerprint-1")
        self.assertTrue(payload["runtimePayload"]["bridgeRunning"])
        self.assertFalse(payload["runtimePayload"]["cloudReachable"])
        self.assertEqual(payload["runtimePayload"]["cloudRetryAt"], "2026-04-06T10:46:21+08:00")
        self.assertEqual(payload["runtimePayload"]["lastError"], "404 Client Error: Not Found")
        self.assertEqual(payload["runtimePayload"]["lastLoginPollAt"], "2026-04-06T10:46:16+08:00")

    def test_request_waits_when_loopback_omnidrive_api_is_unavailable(self):
        bridge = self.make_bridge()
        bridge.cloud_base_url = "http://127.0.0.1:8410"

        with mock.patch.object(agent_module.socket, "create_connection", side_effect=OSError("connection refused")):
            with mock.patch.object(bridge._session, "request") as mock_request:
                with self.assertRaises(agent_module.OmniDriveEndpointUnavailable):
                    bridge._request("POST", "/api/v1/agent/heartbeat", payload={})

        mock_request.assert_not_called()
        status = bridge.status()
        self.assertFalse(status["cloudReachable"])
        self.assertIsNotNone(status["cloudRetryAt"])
        self.assertIn("127.0.0.1:8410", status["lastError"])
        self.assertEqual(status["bridgeStatus"], "degraded")
        self.assertIn("127.0.0.1:8410", status["bridgeLastError"])

    def test_bridge_timeout_marks_bridge_degraded(self):
        bridge = self.make_bridge()

        with mock.patch.object(bridge._session, "request", side_effect=requests.ReadTimeout("read timed out")):
            with self.assertRaises(requests.ReadTimeout):
                bridge._request("GET", "/api/v1/agent/publish-tasks/device-1")

        status = bridge.status()
        self.assertEqual(status["bridgeStatus"], "degraded")
        self.assertIn("read timed out", status["bridgeLastError"])
        self.assertIsNotNone(status["bridgeLastErrorAt"])

    def test_heartbeat_does_not_clear_bridge_degraded_state(self):
        bridge = self.make_bridge()
        bridge._update_state(
            bridgeStatus="degraded",
            bridgeLastError="Read timed out",
            bridgeLastErrorAt="2026-04-07T10:24:37Z",
            lastError="Read timed out",
        )

        with mock.patch.object(agent_module, "get_local_ip", return_value="192.168.1.10"), mock.patch.object(
            bridge, "_request", return_value={"device": {"deviceCode": bridge.device_code}}
        ):
            bridge._heartbeat()

        status = bridge.status()
        self.assertEqual(status["bridgeStatus"], "degraded")
        self.assertEqual(status["bridgeLastError"], "Read timed out")
        self.assertEqual(status["bridgeLastErrorAt"], "2026-04-07T10:24:37Z")

    def test_non_heartbeat_request_recovers_bridge_health(self):
        bridge = self.make_bridge()
        bridge._update_state(
            bridgeStatus="degraded",
            bridgeLastError="Read timed out",
            bridgeLastErrorAt="2026-04-07T10:24:37Z",
        )
        response = mock.Mock()
        response.content = b"{}"
        response.json.return_value = {}
        response.raise_for_status.return_value = None

        with mock.patch.object(bridge._session, "request", return_value=response):
            bridge._request("GET", "/api/v1/agent/publish-tasks/device-1")

        status = bridge.status()
        self.assertEqual(status["bridgeStatus"], "healthy")
        self.assertIsNone(status["bridgeLastError"])
        self.assertIsNone(status["bridgeLastErrorAt"])
        self.assertIsNotNone(status["bridgeLastSuccessAt"])

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

    def test_sync_accounts_deletes_retired_cloud_accounts_locally(self):
        bridge = self.make_bridge()
        self.ensure_user_info_table(bridge.db_path)
        cookie_dir = self.temp_dir / "cookiesFile"
        cookie_dir.mkdir(parents=True, exist_ok=True)
        cookie_file = cookie_dir / "ks-real.json"
        cookie_file.write_text("{}", encoding="utf-8")

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

        request_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            request_calls.append((method, path, payload))
            if method == "GET" and path == "/api/v1/agent/accounts/device-1":
                return {
                    "retiredItems": [
                        {
                            "platform": "快手",
                            "accountName": "测试快手_乔总",
                            "reason": "deleted",
                            "lastChangedAt": "2026-03-23T10:00:00Z",
                        }
                    ]
                }
            if method == "POST" and path == "/api/v1/agent/accounts/retired-ack":
                return {"acked": 1}
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._sync_accounts()

        with sqlite3.connect(bridge.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("SELECT COUNT(*) FROM user_info")
            remaining = cursor.fetchone()[0]

        self.assertEqual(remaining, 0)
        self.assertFalse(cookie_file.exists())
        self.assertEqual(
            request_calls,
            [
                ("GET", "/api/v1/agent/accounts/device-1", None),
                (
                    "POST",
                    "/api/v1/agent/accounts/retired-ack",
                    {
                        "deviceCode": "device-1",
                        "items": [
                            {
                                "platform": "快手",
                                "accountName": "测试快手_乔总",
                                "acknowledgedAt": mock.ANY,
                            }
                        ],
                    },
                ),
            ],
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
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                self.assertEqual(params["limit"], 20)
                return {
                    "items": [
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
                    ],
                    "nextCursor": {"updatedAfter": "2026-04-14T13:00:00Z", "afterId": "cloud-job-1"},
                    "hasMore": False,
                }
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
                with mock.patch.object(bridge, "_sync_generated_material_refs"):
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
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                return {
                    "items": [
                        {
                            "job": {
                                "id": "cloud-job-1",
                                "localTaskId": "local-ai-1",
                                "status": "success",
                                "message": "done",
                            },
                            "artifacts": [{"artifactKey": "video-1", "artifactType": "video"}],
                        }
                    ],
                    "nextCursor": {"updatedAfter": "2026-04-14T13:00:01Z", "afterId": "cloud-job-1"},
                    "hasMore": False,
                }
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
                with mock.patch.object(bridge, "_sync_generated_material_refs") as sync_materials:
                    with mock.patch.object(bridge, "_enqueue_publish_from_ai_task", return_value="publish-1"):
                        imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 1)
        sync_materials.assert_called_once()
        self.assertEqual(delivery_updates[0]["status"], "publish_queued")

    def test_enqueue_publish_from_ai_task_runs_immediately_when_publish_time_has_passed(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        bridge = self.make_bridge(publish_task_manager=publish_task_manager)
        self.ensure_user_info_table(bridge.db_path)

        with sqlite3.connect(bridge.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (4, "kuaishou-real.json", "测试快手_乔总", 1),
            )
            conn.commit()

        overdue_publish_at = (datetime.now(timezone.utc) - timedelta(minutes=20)).replace(microsecond=0)
        overdue_publish_rfc3339 = overdue_publish_at.isoformat().replace("+00:00", "Z")
        local_task = {
            "taskUuid": "local-ai-overdue",
            "cloudJobId": "cloud-ai-overdue",
            "payload": {
                "publishPayload": {
                    "title": "晚到也要发布",
                    "targets": [
                        {
                            "platform": "快手",
                            "accountName": "测试快手_乔总",
                        }
                    ],
                    "runAt": overdue_publish_rfc3339,
                    "requestedRun": overdue_publish_rfc3339,
                    "publishDate": overdue_publish_rfc3339,
                }
            },
        }
        artifact_refs = [
            {
                "root": bridge.generated_root_name,
                "path": "local-ai-overdue/video.mp4",
                "absolutePath": str(self.temp_dir / "generated" / "local-ai-overdue" / "video.mp4"),
                "name": "video.mp4",
                "role": "media",
            }
        ]

        publish_task_uuid = bridge._enqueue_publish_from_ai_task(local_task, artifact_refs)

        self.assertIsNotNone(publish_task_uuid)
        self.assertEqual(len(publish_task_manager.enqueued_specs), 1)
        spec = publish_task_manager.enqueued_specs[0]
        expected_local_run_at = bridge._normalize_datetime(overdue_publish_rfc3339)
        self.assertEqual(spec["status"], "pending")
        self.assertEqual(spec["runAt"], expected_local_run_at)
        self.assertEqual(spec["platformPublishAt"], expected_local_run_at)
        self.assertEqual(spec["payload"]["publishDate"], expected_local_run_at)

    def test_import_remote_ai_jobs_refreshes_payload_before_realigning_linked_publish_task(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        ai_task_manager = DummyAITaskManager()
        bridge = self.make_bridge(
            publish_task_manager=publish_task_manager,
            ai_task_manager=ai_task_manager,
        )

        ai_task_manager.tasks["cloud-job-2"] = {
            "taskUuid": "cloud-job-2",
            "source": "account_skill_binding",
            "jobType": "video",
            "modelName": "veo",
            "skillId": "skill-old",
            "prompt": "旧任务",
            "status": "publish_pending",
            "message": "旧消息",
            "payload": {
                "runAt": "2099-01-02T09:25:01Z",
                "publishAt": "2099-01-02T09:30:01Z",
                "publishPayload": {
                    "title": "酒馆的介绍视频",
                    "runAt": "2099-01-02T09:30:01Z",
                    "requestedRun": "2099-01-02T09:30:01Z",
                    "targets": [{"platform": "抖音", "accountName": "光001"}],
                },
            },
            "cloudJobId": "cloud-job-2",
            "cloudStatus": "scheduled",
            "linkedPublishTaskUuid": "publish-task-1",
            "artifactRefs": [],
        }
        publish_task_manager.tasks["publish-task-1"] = {
            "taskUuid": "publish-task-1",
            "source": "omnidrive_ai",
            "status": "scheduled",
            "message": "等待 AI 产物定时发布",
            "runAt": "2099-01-02 17:30:01",
            "platformPublishAt": "2099-01-02 17:30:01",
        }

        fresh_payload = {
            "runAt": "2099-01-01T11:45:19Z",
            "publishAt": "2099-01-01T11:48:19Z",
            "publishPayload": {
                "title": "酒馆的介绍视频",
                "runAt": "2099-01-01T11:48:19Z",
                "requestedRun": "2099-01-01T11:48:19Z",
                "targets": [{"platform": "抖音", "accountName": "光001"}],
            },
        }

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                return {
                    "items": [
                        {
                            "job": {
                                "id": "cloud-job-2",
                                "status": "success",
                                "source": "account_skill_binding",
                                "jobType": "video",
                                "modelName": "veo-updated",
                                "skillId": "skill-new",
                                "prompt": "新任务",
                                "message": "AI 视频生成完成",
                                "inputPayload": fresh_payload,
                                "localPublishTaskId": "publish-task-1",
                            },
                            "artifacts": [],
                        }
                    ],
                    "nextCursor": {"updatedAfter": "2026-04-14T13:00:02Z", "afterId": "cloud-job-2"},
                    "hasMore": False,
                }
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 0)
        self.assertEqual(
            ai_task_manager.tasks["cloud-job-2"]["payload"]["publishPayload"]["runAt"],
            "2099-01-01T11:48:19Z",
        )
        self.assertEqual(ai_task_manager.tasks["cloud-job-2"]["modelName"], "veo-updated")
        self.assertEqual(
            publish_task_manager.tasks["publish-task-1"]["runAt"],
            "2099-01-01 19:48:19",
        )
        self.assertEqual(
            publish_task_manager.tasks["publish-task-1"]["platformPublishAt"],
            "2099-01-01 19:48:19",
        )

    def test_import_remote_ai_jobs_uses_cloud_publish_binding_and_schedule_times(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        ai_task_manager = DummyAITaskManager()
        bridge = self.make_bridge(
            publish_task_manager=publish_task_manager,
            ai_task_manager=ai_task_manager,
        )

        ai_task_manager.tasks["cloud-job-bound"] = {
            "taskUuid": "cloud-job-bound",
            "source": "account_skill_binding",
            "jobType": "video",
            "modelName": "veo",
            "skillId": "skill-old",
            "prompt": "旧任务",
            "status": "generating",
            "message": "执行中",
            "payload": {"publishPayload": {"title": "旧标题"}},
            "cloudJobId": "cloud-job-bound",
            "cloudStatus": "running",
            "linkedPublishTaskUuid": None,
            "artifactRefs": [],
        }
        publish_task_manager.tasks["publish-task-cloud"] = {
            "taskUuid": "publish-task-cloud",
            "source": "omnidrive_ai",
            "status": "scheduled",
            "message": "等待 AI 产物定时发布",
            "runAt": "2099-01-02 17:30:01",
            "platformPublishAt": "2099-01-02 17:30:01",
        }

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                return {
                    "items": [
                        {
                            "job": {
                                "id": "cloud-job-bound",
                                "status": "success",
                                "source": "account_skill_binding",
                                "jobType": "video",
                                "modelName": "veo-updated",
                                "skillId": "skill-new",
                                "prompt": "新任务",
                                "message": "AI 视频生成完成",
                                "inputPayload": {
                                    "publishPayload": {
                                        "title": "酒馆的介绍视频",
                                        "targets": [{"platform": "抖音", "accountName": "光001"}],
                                    }
                                },
                                "localPublishTaskId": "publish-task-cloud",
                            },
                            "artifacts": [{"artifactKey": "video-1", "artifactType": "video"}],
                            "scheduleTimes": {
                                "createdAt": "2099-01-01T11:00:00Z",
                                "updatedAt": "2099-01-01T11:45:19Z",
                                "generateAt": "2099-01-01T11:45:19Z",
                                "publishAt": "2099-01-01T11:48:19Z",
                                "timezone": "Asia/Shanghai",
                                "timeOfDay": "19:48:19",
                                "repeatDaily": False,
                                "generationLeadMinutes": 3,
                            },
                        }
                    ],
                    "nextCursor": {"updatedAfter": "2026-04-14T13:00:03Z", "afterId": "cloud-job-bound"},
                    "hasMore": False,
                }
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request), \
             mock.patch.object(bridge, "_download_ai_artifacts", side_effect=AssertionError("should not download artifacts when cloud publish task exists")), \
             mock.patch.object(bridge, "_enqueue_publish_from_ai_task", side_effect=AssertionError("should not enqueue local publish when cloud publish task exists")):
            imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 0)
        updated = ai_task_manager.tasks["cloud-job-bound"]
        self.assertEqual(updated["linkedPublishTaskUuid"], "publish-task-cloud")
        self.assertEqual(updated["payload"]["scheduleTimes"]["publishAt"], "2099-01-01T11:48:19Z")
        self.assertEqual(updated["payload"]["publishAt"], "2099-01-01T11:48:19Z")
        self.assertEqual(updated["payload"]["publishPayload"]["runAt"], "2099-01-01T11:48:19Z")
        self.assertEqual(
            publish_task_manager.tasks["publish-task-cloud"]["runAt"],
            "2099-01-01 19:48:19",
        )

    def test_import_remote_ai_jobs_skips_reimport_for_existing_output_ready_results(self):
        publish_task_manager = DummyPublishTaskManager(worker_count=2)
        ai_task_manager = DummyAITaskManager()
        bridge = self.make_bridge(
            publish_task_manager=publish_task_manager,
            ai_task_manager=ai_task_manager,
        )

        ai_task_manager.tasks["cloud-job-3"] = {
            "taskUuid": "cloud-job-3",
            "source": "account_skill_binding",
            "jobType": "video",
            "modelName": "veo",
            "skillId": "skill-old",
            "prompt": "已导入完成",
            "status": "output_ready",
            "message": "AI 产物已回流 OmniBull，本地尚未生成发布任务",
            "payload": {
                "publishPayload": {
                    "title": "旧结果",
                    "targets": [{"platform": "抖音", "accountName": "D001"}],
                },
            },
            "cloudJobId": "cloud-job-3",
            "cloudStatus": "success",
            "linkedPublishTaskUuid": None,
            "artifactRefs": [{"root": "omnidriveGenerated", "path": "cloud-job-3/video.mp4", "role": "media"}],
        }

        request_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            request_calls.append((method, path, payload))
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                return {
                    "items": [
                        {
                            "job": {
                                "id": "cloud-job-3",
                                "status": "success",
                                "source": "account_skill_binding",
                                "jobType": "video",
                                "modelName": "veo",
                                "skillId": "skill-old",
                                "prompt": "已导入完成",
                                "message": "done",
                                "inputPayload": {
                                    "publishPayload": {
                                        "title": "旧结果",
                                        "targets": [{"platform": "抖音", "accountName": "D001"}],
                                    }
                                },
                            },
                            "artifacts": [{"artifactKey": "video-1", "artifactType": "video"}],
                        }
                    ],
                    "nextCursor": {"updatedAfter": "2026-04-14T13:00:04Z", "afterId": "cloud-job-3"},
                    "hasMore": False,
                }
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            with mock.patch.object(bridge, "_download_ai_artifacts") as download_artifacts:
                imported = bridge._import_remote_ai_jobs()

        self.assertEqual(imported, 0)
        download_artifacts.assert_not_called()
        self.assertEqual(len(request_calls), 1)

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

    def test_sync_local_ai_tasks_only_uploads_dirty_tasks_once(self):
        db_path = self.temp_dir / "database.db"
        ai_task_manager = OmniDriveAITaskManager(db_path)
        ai_task_manager.init_db()
        ai_task_manager.create_task(
            {
                "taskUuid": "local-ai-dirty-1",
                "jobType": "video",
                "modelName": "veo",
                "prompt": "local dirty task",
                "inputPayload": {"foo": "bar"},
                "publishPayload": {"title": "dirty"},
            }
        )
        bridge = self.make_bridge(ai_task_manager=ai_task_manager)
        sync_payloads = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "POST" and path == "/api/v1/agent/ai-jobs/sync":
                sync_payloads.append(payload)
                return {"job": {"id": "cloud-local-1", "status": "queued", "message": "ok"}}
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            mirrored_first = bridge._sync_local_ai_tasks()
            mirrored_second = bridge._sync_local_ai_tasks()

        task = ai_task_manager.get_task("local-ai-dirty-1")
        self.assertEqual(mirrored_first, 1)
        self.assertEqual(mirrored_second, 0)
        self.assertEqual(len(sync_payloads), 1)
        self.assertEqual(task["cloudJobId"], "cloud-local-1")
        self.assertFalse(task["cloudSyncDirty"])
        self.assertIsNotNone(task["lastCloudSyncHash"])
        self.assertIsNotNone(task["lastCloudSyncAt"])

    def test_import_remote_ai_jobs_uses_delta_cursor(self):
        bridge = self.make_bridge(ai_task_manager=DummyAITaskManager())
        request_params = []

        def fake_request(method, path, *, params=None, payload=None):
            if method == "GET" and path == "/api/v1/agent/ai-jobs/device-1/delta":
                request_params.append(dict(params or {}))
                if len(request_params) == 1:
                    return {
                        "items": [],
                        "nextCursor": {"updatedAfter": "2026-04-14T13:00:05Z", "afterId": "job-9"},
                        "hasMore": False,
                    }
                return {
                    "items": [],
                    "hasMore": False,
                }
            raise AssertionError(f"unexpected request {method} {path}")

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._import_remote_ai_jobs()
            bridge._import_remote_ai_jobs()

        self.assertEqual(request_params[0], {"limit": 20})
        self.assertEqual(
            request_params[1],
            {"limit": 20, "updatedAfter": "2026-04-14T13:00:05Z", "afterId": "job-9"},
        )
        self.assertEqual(
            bridge.status()["lastAIPollCursor"],
            {"updatedAfter": "2026-04-14T13:00:05Z", "afterId": "job-9"},
        )

    def test_sync_materials_only_uses_generated_root_and_metadata(self):
        materials_root = self.temp_dir / "materials"
        materials_root.mkdir(parents=True, exist_ok=True)
        (materials_root / "ignore.txt").write_text("ignore me", encoding="utf-8")

        generated_root = self.temp_dir / "generated"
        job_dir = generated_root / "job-1"
        job_dir.mkdir(parents=True, exist_ok=True)
        (job_dir / "video.mp4").write_bytes(b"video-bytes")

        bridge = self.make_bridge()
        bridge.material_roots = {
            "materials": materials_root,
            bridge.generated_root_name: generated_root,
        }

        request_calls = []

        def fake_request(method, path, *, params=None, payload=None):
            request_calls.append((method, path, payload))
            return {"ok": True}

        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._sync_materials()

        root_sync_payloads = [payload for method, path, payload in request_calls if path == "/api/v1/agent/materials/roots/sync"]
        directory_payloads = [payload for method, path, payload in request_calls if path == "/api/v1/agent/materials/directory/sync"]
        file_payloads = [payload for method, path, payload in request_calls if path == "/api/v1/agent/materials/file/sync"]

        self.assertTrue(root_sync_payloads)
        self.assertEqual({item["name"] for item in root_sync_payloads[0]["roots"]}, {bridge.generated_root_name})
        self.assertTrue(directory_payloads)
        self.assertEqual({payload["root"] for payload in directory_payloads}, {bridge.generated_root_name})
        self.assertEqual(len(file_payloads), 1)
        self.assertEqual(file_payloads[0]["root"], bridge.generated_root_name)
        self.assertIsNone(file_payloads[0]["previewText"])
        self.assertFalse(file_payloads[0]["isText"])

        request_calls.clear()
        with mock.patch.object(bridge, "_request", side_effect=fake_request):
            bridge._sync_materials()

        repeated_directory_payloads = [payload for method, path, payload in request_calls if path == "/api/v1/agent/materials/directory/sync"]
        repeated_file_payloads = [payload for method, path, payload in request_calls if path == "/api/v1/agent/materials/file/sync"]
        self.assertEqual(repeated_directory_payloads, [])
        self.assertEqual(repeated_file_payloads, [])

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

    def test_local_ai_status_from_remote_job_maps_waiting_recharge(self):
        status = agent_module.OmniDriveBridge._local_ai_status_from_remote_job(
            {
                "status": "waiting_recharge",
                "deliveryStatus": "",
            }
        )

        self.assertEqual(status, "waiting_recharge")


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

    def test_publish_task_manager_does_not_realign_generic_failed_omnidrive_ai_tasks(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-no-realign-failed-"))
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
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    "publish-no-realign",
                    "omnidrive_ai",
                    3,
                    "抖音",
                    "D001",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "generic failed task",
                    "2026-03-21 17:03:49",
                    "2026-03-21 17:03:49",
                    "failed",
                    "发布任务执行失败: 本地未找到账号: 抖音 / D001",
                    json.dumps({"omnidriveAITaskUuid": "local-ai-1"}, ensure_ascii=False),
                ),
            )
            conn.commit()

        changed = manager.realign_omnidrive_ai_task(
            "publish-no-realign",
            "2026-03-21T09:03:49Z",
            "2026-03-21T09:03:49Z",
        )
        task = manager.get_task("publish-no-realign")

        self.assertFalse(changed)
        self.assertEqual(task["status"], "failed")
        self.assertEqual(task["message"], "发布任务执行失败: 本地未找到账号: 抖音 / D001")

    def test_publish_task_manager_realigns_browser_closed_omnidrive_ai_failures_after_restart(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-browser-closed-realign-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        future_publish_at = datetime.now(timezone.utc).replace(microsecond=0) + timedelta(hours=1)

        ai_manager = OmniDriveAITaskManager(db_path)
        ai_manager.init_db()
        ai_manager.create_task(
            {
                "taskUuid": "local-ai-browser-closed",
                "jobType": "video",
                "modelName": "veo-3.1-fast-fl",
                "prompt": "浏览器关闭恢复测试",
                "runAt": future_publish_at.isoformat().replace("+00:00", "Z"),
                "publishPayload": {
                    "runAt": future_publish_at.isoformat().replace("+00:00", "Z"),
                    "requestedRun": future_publish_at.isoformat().replace("+00:00", "Z"),
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
                    "publish-browser-closed",
                    "omnidrive_ai",
                    3,
                    "抖音",
                    "测试账号",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:local-ai-browser-closed/video.mp4",
                    "browser closed recover",
                    broken_local_run_at,
                    broken_local_run_at,
                    "failed",
                    "发布任务执行失败: Locator.count: Target page, context or browser has been closed",
                    json.dumps({"omnidriveAITaskUuid": "local-ai-browser-closed"}, ensure_ascii=False),
                ),
            )
            conn.commit()

        manager.init_db()
        repaired_task = manager.get_task("publish-browser-closed")
        expected_local_run_at = future_publish_at.astimezone().strftime("%Y-%m-%d %H:%M:%S")

        self.assertEqual(repaired_task["status"], "scheduled")
        self.assertEqual(repaired_task["runAt"], expected_local_run_at)
        self.assertEqual(repaired_task["platformPublishAt"], expected_local_run_at)
        self.assertEqual(repaired_task["message"], "等待 AI 产物定时发布")
        self.assertIsNone(repaired_task["startedAt"])
        self.assertIsNone(repaired_task["finishedAt"])

    def test_publish_task_manager_uses_local_schedule_for_enable_timer_tasks(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-local-schedule-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        OmniDriveBridgeTests.ensure_user_info_table(db_path)
        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (3, "douyin-real.json", "抖音测试号", 1),
            )
            conn.commit()

        scheduled_publish_at = datetime.now(timezone.utc).replace(microsecond=0) + timedelta(hours=2)
        expected_local_run_at = PublishTaskManager._normalize_datetime(scheduled_publish_at)
        with mock.patch(
            "utils.publish_task_manager.generate_schedule_time_next_day",
            return_value=[scheduled_publish_at],
        ):
            specs = manager._build_task_specs(
                {
                    "type": 3,
                    "title": "本地定时发布",
                    "tags": ["测试"],
                    "fileList": ["demo/video.mp4"],
                    "accountList": ["douyin-real.json"],
                    "enableTimer": 1,
                    "videosPerDay": 1,
                    "dailyTimes": ["16:30"],
                    "startDays": 0,
                },
                source="local_api",
            )

        self.assertEqual(len(specs), 1)
        self.assertEqual(specs[0]["status"], "scheduled")
        self.assertEqual(specs[0]["runAt"], expected_local_run_at)
        self.assertEqual(specs[0]["platformPublishAt"], expected_local_run_at)
        self.assertEqual(specs[0]["payload"]["publishDate"], expected_local_run_at)

    def test_publish_task_manager_executes_platform_publish_immediately(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-immediate-publish-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        captured = {}

        class FakeDouYinVideo:
            def __init__(self, title, file_path, tags, publish_date, account_file, thumbnail_path, product_link, product_title):
                captured["title"] = title
                captured["filePath"] = file_path
                captured["tags"] = tags
                captured["publishDate"] = publish_date
                captured["accountFile"] = account_file
                captured["thumbnailPath"] = thumbnail_path
                captured["productLink"] = product_link
                captured["productTitle"] = product_title

            async def main(self):
                return None

        payload = {
            "platformType": 3,
            "platformName": "抖音",
            "title": "立即发布测试",
            "tags": ["即时"],
            "filePath": "generated:job-1/video.mp4",
            "accountFilePath": "douyin-real.json",
            "accountName": "抖音测试号",
            "publishDate": "2026-03-22 18:30:00",
            "fileSourceMode": "material",
            "materialRoot": "generated",
            "materialPath": "job-1/video.mp4",
            "sourceAbsolutePath": "/tmp/video.mp4",
            "thumbnailSourceMode": "material",
            "thumbnailRoot": "generated",
            "thumbnailPath": "job-1/thumb.png",
            "thumbnailAbsolutePath": "/tmp/thumb.png",
            "productLink": "https://example.com/product",
            "productTitle": "商品标题",
        }

        with mock.patch.object(manager, "_resolve_account_binding", return_value=("douyin-real.json", "抖音测试号", {})):
            with mock.patch.object(manager, "_resolve_payload_file_path", return_value="/tmp/video.mp4"):
                with mock.patch.object(manager, "_resolve_thumbnail_path", return_value="/tmp/thumb.png"):
                    with mock.patch("utils.publish_task_manager.account_storage_exists", return_value=True):
                        with mock.patch("utils.publish_task_manager.DouYinVideo", FakeDouYinVideo):
                            manager._execute_payload(payload)

        self.assertEqual(captured["title"], "立即发布测试")
        self.assertEqual(captured["publishDate"], 0)
        self.assertEqual(captured["accountFile"], "douyin-real.json")

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

    def test_publish_task_manager_requeues_browser_closed_errors_immediately(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-browser-closed-immediate-"))
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
                    worker_name, started_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
                """,
                (
                    "publish-browser-closed-immediate",
                    "omnidrive_ai",
                    4,
                    "快手",
                    "测试快手_乔总",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "browser close immediate retry",
                    None,
                    None,
                    "running",
                    "任务执行中",
                    json.dumps({}, ensure_ascii=False),
                    "worker-1",
                ),
            )
            conn.commit()

        task = manager.get_task("publish-browser-closed-immediate")
        with mock.patch.object(
            manager,
            "_execute_payload",
            side_effect=RuntimeError("Locator.count: Target page, context or browser has been closed"),
        ):
            manager._run_task(task)

        retried_task = manager.get_task("publish-browser-closed-immediate")
        self.assertEqual(retried_task["status"], "pending")
        self.assertEqual(retried_task["message"], "浏览器意外关闭，准备自动重试")
        self.assertIsNone(retried_task["workerName"])
        self.assertIsNone(retried_task["startedAt"])
        self.assertIsNone(retried_task["finishedAt"])
        self.assertEqual(retried_task["autoRetryCount"], 1)

    def test_publish_task_manager_stops_after_automatic_interrupt_retry_is_exhausted(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-browser-closed-cap-"))
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
                    worker_name, started_at, auto_retry_count
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
                """,
                (
                    "publish-browser-closed-cap",
                    "omnidrive_ai",
                    4,
                    "快手",
                    "测试快手_乔总",
                    "cookies/demo.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "browser close retry cap",
                    None,
                    None,
                    "running",
                    "任务执行中",
                    json.dumps({}, ensure_ascii=False),
                    "worker-1",
                    1,
                ),
            )
            conn.commit()

        task = manager.get_task("publish-browser-closed-cap")
        with mock.patch.object(
            manager,
            "_execute_payload",
            side_effect=RuntimeError("Locator.count: Target page, context or browser has been closed"),
        ):
            manager._run_task(task)

        failed_task = manager.get_task("publish-browser-closed-cap")
        self.assertEqual(failed_task["status"], "failed")
        self.assertEqual(failed_task["message"], "浏览器或执行环境连续中断，已停止自动重试，请人工处理后重试")
        self.assertEqual(failed_task["autoRetryCount"], 1)
        self.assertIsNotNone(failed_task["finishedAt"])

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

    def test_publish_task_manager_cleans_generated_materials_when_all_related_tasks_succeed(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-clean-generated-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        generated_root = temp_dir / "omnidriveSync" / "generated"
        generated_dir = generated_root / "ai-task-1"
        generated_dir.mkdir(parents=True, exist_ok=True)
        (generated_dir / "video.mp4").write_bytes(b"video")
        (generated_dir / "cover.png").write_bytes(b"cover")

        manager = PublishTaskManager(
            db_path=db_path,
            material_roots={"omnidriveGenerated": generated_root},
        )
        manager.init_db()

        payload = {
            "omnidriveAITaskUuid": "ai-task-1",
            "omnidriveMaterialRefs": [
                {"root": "omnidriveGenerated", "path": "ai-task-1/video.mp4", "absolutePath": str(generated_dir / "video.mp4")},
                {"root": "omnidriveGenerated", "path": "ai-task-1/cover.png", "absolutePath": str(generated_dir / "cover.png")},
            ],
            "sourceAbsolutePath": str(generated_dir / "video.mp4"),
            "thumbnailAbsolutePath": str(generated_dir / "cover.png"),
        }

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.executemany(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                [
                    (
                        "publish-1",
                        "omnidrive_ai",
                        3,
                        "抖音",
                        "账号A",
                        "cookies/a.json",
                        "video.mp4",
                        "omnidriveGenerated:ai-task-1/video.mp4",
                        "发布A",
                        None,
                        None,
                        "success",
                        "ok",
                        json.dumps(payload, ensure_ascii=False),
                    ),
                    (
                        "publish-2",
                        "omnidrive_ai",
                        4,
                        "快手",
                        "账号B",
                        "cookies/b.json",
                        "video.mp4",
                        "omnidriveGenerated:ai-task-1/video.mp4",
                        "发布B",
                        None,
                        None,
                        "success",
                        "ok",
                        json.dumps(payload, ensure_ascii=False),
                    ),
                ],
            )
            conn.commit()

        cleaned = manager._cleanup_published_generated_materials()

        self.assertGreaterEqual(cleaned, 1)
        self.assertFalse(generated_dir.exists())

    def test_publish_task_manager_keeps_generated_materials_when_related_tasks_not_all_success(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-keep-generated-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        generated_root = temp_dir / "omnidriveSync" / "generated"
        generated_dir = generated_root / "ai-task-2"
        generated_dir.mkdir(parents=True, exist_ok=True)
        media_path = generated_dir / "video.mp4"
        media_path.write_bytes(b"video")

        manager = PublishTaskManager(
            db_path=db_path,
            material_roots={"omnidriveGenerated": generated_root},
        )
        manager.init_db()

        payload = {
            "omnidriveAITaskUuid": "ai-task-2",
            "omnidriveMaterialRefs": [
                {"root": "omnidriveGenerated", "path": "ai-task-2/video.mp4", "absolutePath": str(media_path)},
            ],
            "sourceAbsolutePath": str(media_path),
        }

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.executemany(
                """
                INSERT INTO publish_tasks (
                    task_uuid, source, platform_type, platform_name, account_name, account_file_path,
                    file_name, file_path, title, run_at, platform_publish_at, status, message, payload_json
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                [
                    (
                        "publish-3",
                        "omnidrive_ai",
                        3,
                        "抖音",
                        "账号A",
                        "cookies/a.json",
                        "video.mp4",
                        "omnidriveGenerated:ai-task-2/video.mp4",
                        "发布A",
                        None,
                        None,
                        "success",
                        "ok",
                        json.dumps(payload, ensure_ascii=False),
                    ),
                    (
                        "publish-4",
                        "omnidrive_ai",
                        4,
                        "快手",
                        "账号B",
                        "cookies/b.json",
                        "video.mp4",
                        "omnidriveGenerated:ai-task-2/video.mp4",
                        "发布B",
                        None,
                        None,
                        "pending",
                        "waiting",
                        json.dumps(payload, ensure_ascii=False),
                    ),
                ],
            )
            conn.commit()

        cleaned = manager._cleanup_published_generated_materials()

        self.assertEqual(cleaned, 0)
        self.assertTrue(media_path.exists())

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

    def test_ai_task_manager_imports_waiting_recharge_without_finishing_task(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="ai-task-manager-waiting-recharge-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = OmniDriveAITaskManager(db_path)
        manager.init_db()

        task = manager.import_remote_task(
            {
                "taskUuid": "local-ai-waiting-recharge",
                "source": "account_skill_binding",
                "jobType": "video",
                "modelName": "veo-3.1-fast-fl",
                "prompt": "生成玩具短视频",
                "cloudStatus": "waiting_recharge",
                "message": "当前积分不足，充值后会自动继续执行",
                "payload": {},
            }
        )

        self.assertEqual(task["status"], "waiting_recharge")
        self.assertIsNone(task["finishedAt"])

    def test_ai_task_manager_update_cloud_binding_preserves_local_publish_progress(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="ai-task-manager-cloud-binding-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = OmniDriveAITaskManager(db_path)
        manager.init_db()

        manager.import_remote_task(
            {
                "taskUuid": "local-ai-publish-progress",
                "source": "omnidrive_cloud",
                "jobType": "video",
                "modelName": "veo-3.1-fast-fl",
                "prompt": "生成玩具短视频",
                "status": "publish_pending",
                "cloudStatus": "success",
                "message": "AI 产物已回流 OmniBull，并进入 SAU 发布队列",
                "payload": {},
                "cloudJobId": "cloud-job-1",
                "linkedPublishTaskUuid": "publish-task-1",
            }
        )

        updated = manager.update_cloud_binding(
            "local-ai-publish-progress",
            "cloud-job-1",
            "success",
            "云端轮询再次对齐",
        )

        self.assertEqual(updated["status"], "publish_pending")
        self.assertEqual(updated["cloudStatus"], "success")

    def test_ai_task_manager_serializes_cloud_schedule_times(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="ai-task-manager-schedule-times-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = OmniDriveAITaskManager(db_path)
        manager.init_db()

        schedule_times = {
            "createdAt": "2099-01-01T11:00:00Z",
            "updatedAt": "2099-01-01T11:45:19Z",
            "generateAt": "2099-01-01T11:45:19Z",
            "publishAt": "2099-01-01T11:48:19Z",
            "timezone": "Asia/Shanghai",
            "timeOfDay": "19:48:19",
            "repeatDaily": False,
            "generationLeadMinutes": 3,
        }
        manager.import_remote_task(
            {
                "taskUuid": "cloud-ai-schedule-times",
                "source": "account_skill_binding",
                "jobType": "video",
                "modelName": "veo-3.1-fast-fl",
                "prompt": "生成玩具短视频",
                "status": "scheduled",
                "cloudStatus": "scheduled",
                "message": "等待定时执行",
                "payload": {
                    "scheduleTimes": schedule_times,
                    "publishPayload": {
                        "title": "排程视频",
                    },
                },
                "cloudJobId": "cloud-job-schedule-times",
            }
        )

        task = manager.get_task("cloud-ai-schedule-times")

        self.assertEqual(task["scheduleTimes"]["publishAt"], "2099-01-01T11:48:19Z")
        self.assertEqual(task["createdAt"], "2099-01-01T11:00:00Z")
        self.assertEqual(task["updatedAt"], "2099-01-01T11:45:19Z")
        self.assertEqual(task["generateAt"], "2099-01-01T11:45:19Z")
        self.assertEqual(task["publishAt"], "2099-01-01T11:48:19Z")
        self.assertIsNotNone(task["localCreatedAt"])
        self.assertIsNotNone(task["localUpdatedAt"])

    def test_publish_task_manager_forces_single_worker(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-single-worker-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = PublishTaskManager(db_path=db_path, worker_count=3, material_roots={})

        self.assertEqual(manager.worker_count, 1)
        self.assertEqual(manager.configured_worker_count, 3)

    def test_publish_task_manager_defers_claim_until_dispatch_interval_elapsed(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="publish-task-manager-dispatch-interval-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"
        manager = PublishTaskManager(
            db_path=db_path,
            material_roots={},
            dispatch_interval_seconds=5,
        )
        manager.init_db()

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
                    "publish-dispatch-interval-1",
                    "omnidrive_ai",
                    3,
                    "抖音",
                    "光001",
                    "cookies/guang001.json",
                    "video.mp4",
                    "generated:job-1/video.mp4",
                    "dispatch gate test",
                    None,
                    None,
                    "pending",
                    "等待发布",
                    json.dumps({}, ensure_ascii=False),
                ),
            )
            conn.commit()

        manager._next_dispatch_after_monotonic = 100.0
        with mock.patch("utils.publish_task_manager.time.monotonic", return_value=99.0):
            blocked = manager._claim_next_ready_task("worker-1")
        self.assertIsNone(blocked)

        with mock.patch("utils.publish_task_manager.time.monotonic", side_effect=[100.5, 100.5]):
            claimed = manager._claim_next_ready_task("worker-1")

        self.assertIsNotNone(claimed)
        self.assertEqual(claimed["taskUuid"], "publish-dispatch-interval-1")
        self.assertAlmostEqual(manager._next_dispatch_after_monotonic, 105.5)

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


class OmniDriveAITaskManagerStateTests(unittest.TestCase):
    def test_update_cloud_binding_clears_publish_link_after_requeue(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="omnidrive-ai-task-requeue-reset-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = OmniDriveAITaskManager(db_path)
        manager.init_db()
        manager.import_remote_task(
            {
                "taskUuid": "local-ai-reset",
                "source": "omnibull_local",
                "jobType": "video",
                "modelName": "veo-3.1-fast-fl",
                "prompt": "reset publish bridge",
                "status": "publish_pending",
                "cloudStatus": "completed",
                "message": "AI 产物已回流",
                "payload": {"publishPayload": {"runAt": "2026-03-30T10:00:00Z"}},
                "cloudJobId": "cloud-job-1",
                "linkedPublishTaskUuid": "publish-task-1",
                "artifactRefs": [{"root": "generated", "path": "job/video.mp4"}],
            }
        )

        updated = manager.update_cloud_binding(
            "local-ai-reset",
            "cloud-job-1",
            "queued",
            "AI 任务已重新排队",
            payload={"publishPayload": {"runAt": "2026-03-30T10:00:00Z"}},
        )

        self.assertEqual(updated["status"], "queued_cloud")
        self.assertEqual(updated["cloudStatus"], "queued")
        self.assertIsNone(updated["linkedPublishTaskUuid"])
        self.assertEqual(updated["artifactRefs"], [])
        self.assertIsNone(updated["finishedAt"])

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
