import json
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import requests

from utils import omnidrive_agent as agent_module


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
    def summary(self):
        return {}


class OmniDriveBridgeTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = Path(tempfile.mkdtemp(prefix="omnidrive-agent-test-"))
        self.addCleanup(lambda: shutil.rmtree(self.temp_dir, ignore_errors=True))

    def make_bridge(self, publish_task_manager=None):
        publish_task_manager = publish_task_manager or DummyPublishTaskManager()
        with mock.patch.object(agent_module, "BASE_DIR", self.temp_dir):
            bridge = agent_module.OmniDriveBridge(
                db_path=self.temp_dir / "database.db",
                cloud_base_url="https://cloud.test",
                agent_key="agent-key",
                publish_task_manager=publish_task_manager,
                ai_task_manager=DummyAITaskManager(),
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
