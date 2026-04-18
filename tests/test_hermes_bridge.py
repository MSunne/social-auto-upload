import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

os.environ.setdefault("FLASK_RUN_FROM_CLI", "true")
os.environ.setdefault("FLASK_DEBUG", "true")
os.environ.setdefault("WERKZEUG_RUN_MAIN", "false")

import sau_backend
import utils.publish_task_manager as publish_task_manager_module
from utils.omnidrive_ai_task_manager import OmniDriveAITaskManager
from utils.publish_task_manager import PublishTaskManager


class HermesSharedRuntimeTests(unittest.TestCase):
    def test_sync_shared_runtime_config_uses_single_source_defaults(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            runtime_path = Path(temp_dir) / "runtime" / "hermes-openclaw-runtime.json"
            with mock.patch.object(sau_backend, "HERMES_SHARED_RUNTIME_CONFIG_PATH", runtime_path):
                payload, paths = sau_backend.sync_hermes_shared_runtime_config(
                    model_items=[
                        {"modelName": "gpt-5.4"},
                        {"modelName": "gpt-image-1"},
                        {"modelName": "gpt-5.4"},
                    ],
                    api_base_url="https://cloud.example.com",
                    access_token="secret-token",
                    device={
                        "id": "device-1",
                        "deviceCode": "device-code-1",
                        "name": "Factory OmniBull",
                        "defaultChatModel": "gpt-5.4",
                        "defaultImageModel": "gpt-image-1",
                        "defaultVideoModel": "veo-3",
                    },
                )

            self.assertEqual(paths, [str(runtime_path)])
            self.assertTrue(runtime_path.exists())
            self.assertEqual(payload["provider"]["baseUrl"], "https://cloud.example.com/openai/v1")
            self.assertEqual(payload["provider"]["apiKey"], "secret-token")
            self.assertEqual(payload["defaults"]["chatModel"], "gpt-5.4")
            self.assertEqual(payload["defaults"]["imageModel"], "gpt-image-1")
            self.assertEqual(payload["defaults"]["videoModel"], "veo-3")
            self.assertEqual(payload["routing"]["mode"], "shared_runtime_single_source")
            self.assertEqual(payload["models"], ["gpt-5.4", "gpt-image-1"])

            saved = json.loads(runtime_path.read_text(encoding="utf-8"))
            self.assertEqual(saved["defaults"]["chatModel"], "gpt-5.4")
            self.assertEqual(
                sau_backend._sanitize_shared_runtime_config_for_response(saved)["provider"]["apiKey"],
                "[REDACTED]",
            )


class HermesBridgeRouteTests(unittest.TestCase):
    def test_status_route_reports_redacted_shared_runtime(self):
        shared_runtime = {
            "provider": {
                "baseUrl": "http://127.0.0.1:5409/openai/v1",
                "apiKey": "secret-token",
            },
            "defaults": {
                "chatModel": "gpt-5.4",
                "imageModel": "gpt-image-1",
                "videoModel": "veo-3",
            },
        }

        def fake_hermes_request(method, path, **kwargs):
            if path == "/health":
                return 200, {"status": "ok"}
            if path == "/v1/models":
                return 200, {"data": [{"id": "gpt-5.4"}]}
            raise AssertionError(f"unexpected path: {path}")

        with mock.patch.object(sau_backend, "ensure_omnidrive_agent_started"), mock.patch.object(
            sau_backend, "ensure_openclaw_omnidrive_runtime_sync_started"
        ), mock.patch.object(
            sau_backend,
            "refresh_openclaw_omnidrive_runtime_config",
            return_value={"sharedRuntime": shared_runtime},
        ), mock.patch.object(
            sau_backend,
            "hermes_api_json_request",
            side_effect=fake_hermes_request,
        ):
            with sau_backend.app.test_client() as client:
                response = client.get("/api/hermes/status?correlationId=cid-status")

        self.assertEqual(response.status_code, 200)
        payload = response.get_json()["data"]
        self.assertTrue(payload["bridge"]["ready"])
        self.assertTrue(payload["hermesApi"]["reachable"])
        self.assertEqual(payload["correlationId"], "cid-status")
        self.assertEqual(payload["sharedRuntime"]["provider"]["apiKey"], "[REDACTED]")
        self.assertEqual(payload["sharedRuntime"]["defaults"]["chatModel"], "gpt-5.4")

    def test_chat_route_falls_back_to_omnidrive_when_hermes_fails(self):
        with mock.patch.object(
            sau_backend,
            "_run_hermes_chat_request",
            side_effect=RuntimeError("hermes-down"),
        ) as hermes_mock, mock.patch.object(
            sau_backend,
            "_run_omnidrive_direct_chat",
            return_value={
                "source": "openclaw_via_hermes",
                "executionEngine": "openclaw_direct",
                "correlationId": "cid-chat",
                "fallback": False,
                "text": "fallback-ok",
                "jobId": "job-1",
                "modelName": "gpt-5.4",
            },
        ) as fallback_mock:
            with sau_backend.app.test_client() as client:
                response = client.post(
                    "/api/hermes/chat",
                    json={
                        "prompt": "今天有哪些待执行任务？",
                        "source": "openclaw_via_hermes",
                        "correlationId": "cid-chat",
                    },
                )

        self.assertEqual(response.status_code, 200)
        payload = response.get_json()["data"]
        self.assertTrue(payload["fallback"])
        self.assertEqual(payload["text"], "fallback-ok")
        self.assertEqual(payload["source"], "openclaw_via_hermes")
        self.assertIn("hermes-down", payload["fallbackReason"])
        hermes_mock.assert_called_once()
        fallback_mock.assert_called_once_with(
            {
                "prompt": "今天有哪些待执行任务？",
                "source": "openclaw_via_hermes",
                "correlationId": "cid-chat",
            },
            "openclaw_via_hermes",
            "cid-chat",
        )

    def test_task_run_route_keeps_source_and_engine_metadata(self):
        with mock.patch.object(
            sau_backend,
            "_run_bridge_skill",
            return_value={"items": [{"accountId": 1}]},
        ) as run_skill_mock:
            with sau_backend.app.test_client() as client:
                response = client.post(
                    "/api/hermes/task/run",
                    json={
                        "taskType": "run_skill",
                        "skillName": "omnibull-accounts",
                        "action": "list",
                        "source": "openclaw_via_hermes",
                        "correlationId": "cid-run",
                    },
                )

        self.assertEqual(response.status_code, 200)
        payload = response.get_json()["data"]
        self.assertEqual(payload["source"], "openclaw_via_hermes")
        self.assertEqual(payload["executionEngine"], "hermes")
        self.assertEqual(payload["correlationId"], "cid-run")
        self.assertEqual(payload["items"][0]["accountId"], 1)
        run_skill_mock.assert_called_once_with(
            {
                "taskType": "run_skill",
                "skillName": "omnibull-accounts",
                "action": "list",
                "source": "openclaw_via_hermes",
                "correlationId": "cid-run",
            },
            source_category="openclaw_via_hermes",
            execution_engine="hermes",
            correlation_id="cid-run",
        )


class HermesTaskMetadataTests(unittest.TestCase):
    def test_ai_task_manager_exposes_source_execution_and_correlation(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            manager = OmniDriveAITaskManager(Path(temp_dir) / "omnidrive_ai.db")
            manager.start()
            task = manager.create_task(
                {
                    "jobType": "image",
                    "modelName": "gpt-image-1",
                    "prompt": "generate a cover",
                    "sourceCategory": "hermes_direct",
                    "executionEngine": "hermes",
                    "correlationId": "cid-ai",
                },
                source="hermes_direct",
            )

        self.assertEqual(task["source"], "hermes_direct")
        self.assertEqual(task["sourceCategory"], "hermes_direct")
        self.assertEqual(task["executionEngine"], "hermes")
        self.assertEqual(task["correlationId"], "cid-ai")

    def test_publish_task_manager_exposes_source_execution_and_correlation(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_root = Path(temp_dir)
            with mock.patch.object(
                publish_task_manager_module,
                "BASE_DIR",
                temp_root,
            ), mock.patch.object(
                publish_task_manager_module,
                "ensure_platform_operation_enabled",
                return_value={"label": "抖音"},
            ):
                manager = PublishTaskManager(temp_root / "publish.db", worker_count=1, retention_days=1)
                manager.init_db()

                with mock.patch.object(
                    manager,
                    "_normalize_file_inputs",
                    return_value=[{"mode": "video_record", "filePath": "videos/demo.mp4"}],
                ), mock.patch.object(
                    manager,
                    "_load_file_records",
                    return_value={"videos/demo.mp4": {"filename": "demo.mp4"}},
                ), mock.patch.object(
                    manager,
                    "_load_account_records",
                    return_value={"cookies/account.json": {"userName": "测试账号"}},
                ), mock.patch.object(
                    manager,
                    "_resolve_account_binding",
                    return_value=("cookies/account.json", "测试账号", None),
                ):
                    tasks = manager.enqueue_from_request(
                        {
                            "type": 3,
                            "title": "Hermes 编排发布",
                            "fileList": ["videos/demo.mp4"],
                            "accountList": ["cookies/account.json"],
                            "sourceCategory": "openclaw_via_hermes",
                            "executionEngine": "hermes",
                            "correlationId": "cid-publish",
                        },
                        source="openclaw_skill",
                    )
                    task = manager.get_task(tasks[0]["taskUuid"])

        self.assertEqual(len(tasks), 1)
        self.assertEqual(task["source"], "openclaw_skill")
        self.assertEqual(task["sourceCategory"], "openclaw_via_hermes")
        self.assertEqual(task["executionEngine"], "hermes")
        self.assertEqual(task["correlationId"], "cid-publish")


if __name__ == "__main__":
    unittest.main()
