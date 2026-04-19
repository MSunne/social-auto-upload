import os
import json
import tempfile
import unittest
import sqlite3
from datetime import datetime
from pathlib import Path
from queue import Queue
from unittest import mock

os.environ.setdefault("FLASK_RUN_FROM_CLI", "true")
os.environ.setdefault("FLASK_DEBUG", "true")
os.environ.setdefault("WERKZEUG_RUN_MAIN", "false")

import sau_backend


class OmniDriveOpenAIProxyHelpersTests(unittest.TestCase):
    def test_local_tool_guard_requires_explicit_local_operation(self):
        self.assertFalse(
            sau_backend._prompt_requests_local_system_action(
                "安装wecom-app,@openclaw-china/wecom和OpenClaw Lark/Feishu Plugin"
            )
        )
        self.assertTrue(
            sau_backend._prompt_requests_local_system_action("请帮我读取本地文件并修改配置")
        )
        self.assertTrue(
            sau_backend._prompt_requests_local_system_action("please run command and restart service")
        )

    def test_chat_model_alias_map_supports_dot_and_dash_variants(self):
        alias_map = sau_backend._build_omnidrive_chat_model_alias_map(
            [
                {"modelName": "gpt-5.4", "id": "model-gpt"},
                {"modelName": "qwen3.5-plus", "id": "model-qwen"},
            ]
        )

        self.assertEqual(alias_map["gpt-5.4"], "gpt-5.4")
        self.assertEqual(alias_map["gpt-5-4"], "gpt-5.4")
        self.assertEqual(alias_map["qwen3.5-plus"], "qwen3.5-plus")
        self.assertEqual(alias_map["qwen3-5-plus"], "qwen3.5-plus")

    def test_sync_openclaw_configs_prunes_legacy_omnidrive_aliases(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            config_path.write_text(
                json.dumps(
                    {
                        "models": {
                            "providers": {
                                "omnidrive": {
                                    "baseUrl": "http://127.0.0.1:8410/openai/v1",
                                    "api": "openai-completions",
                                    "models": [],
                                    "apiKey": "expired-token",
                                }
                            }
                        },
                        "agents": {
                            "defaults": {
                                "models": {
                                    "omnidrive/gpt-5-4": {"alias": "old-omni-gpt"},
                                    "omnidrive/default-chat": {"alias": "old-omni"},
                                }
                            }
                        },
                    }
                ),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path,)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 5409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", ""):
                    sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "gpt-5.4"}, {"modelName": "qwen3.5-plus"}],
                        api_base_url="http://127.0.0.1:8410",
                        access_token="fresh-token",
                        default_chat_model="gpt-5.4",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            data = json.loads(config_path.read_text(encoding="utf-8"))
            defaults = data["agents"]["defaults"]["models"]
            provider = data["models"]["providers"]["omnidrive"]

            self.assertNotIn("omnidrive/gpt-5-4", defaults)
            self.assertNotIn("omnidrive/default-chat", defaults)
            self.assertEqual(defaults["omnidrive/gpt-5.4"]["alias"], "omni")
            self.assertEqual(defaults["omnidrive/qwen3.5-plus"]["alias"], "omni-qwen")
            self.assertEqual(provider["baseUrl"], "http://127.0.0.1:5409/openai/v1")
            self.assertEqual(provider["api"], "openai-completions")
            self.assertNotIn("apiKey", provider)
            self.assertEqual([item["id"] for item in provider["models"]], ["gpt-5.4", "qwen3.5-plus"])

    def test_sync_openclaw_configs_overwrites_model_cache_with_online_set(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            config_path.write_text(
                json.dumps(
                    {
                        "models": {
                            "providers": {
                                "omnidrive": {
                                    "baseUrl": "https://aitoplus.com/openai/v1",
                                    "api": "openai-completions",
                                    "models": [
                                        {"id": "gpt-5.4"},
                                        {"id": "qwen3.5-plus"},
                                    ],
                                    "apiKey": "expired-token",
                                }
                            }
                        },
                        "agents": {
                            "defaults": {
                                "models": {
                                    "omnidrive/gpt-5.4": {"alias": "omni"},
                                    "omnidrive/qwen3.5-plus": {"alias": "omni-qwen"},
                                }
                            }
                        },
                    }
                ),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path,)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 5409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", ""):
                    sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "claude-opus-4-6-thinking"}],
                        api_base_url="https://aitoplus.com",
                        access_token="fresh-token",
                        default_chat_model="claude-opus-4-6-thinking",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            data = json.loads(config_path.read_text(encoding="utf-8"))
            provider = data["models"]["providers"]["omnidrive"]
            defaults = data["agents"]["defaults"]["models"]

            self.assertEqual(provider["baseUrl"], "http://127.0.0.1:5409/openai/v1")
            self.assertEqual(provider["api"], "openai-completions")
            self.assertNotIn("apiKey", provider)
            self.assertEqual([item["id"] for item in provider["models"]], ["claude-opus-4-6-thinking"])
            self.assertEqual(defaults, {"omnidrive/claude-opus-4-6-thinking": {"alias": "omni"}})

    def test_sync_openclaw_configs_exposes_every_online_model_in_defaults(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            config_path.write_text(
                json.dumps(
                    {
                        "models": {
                            "providers": {
                                "omnidrive": {
                                    "baseUrl": "https://aitoplus.com/openai/v1",
                                    "api": "openai-completions",
                                    "models": [],
                                    "apiKey": "expired-token",
                                }
                            }
                        },
                        "agents": {"defaults": {"models": {}}},
                    }
                ),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path,)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 5409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", ""):
                    sau_backend.sync_openclaw_omnidrive_model_configs(
                        [
                            {"modelName": "claude-opus-4-6-thinking"},
                            {"modelName": "deepseek-chat"},
                            {"modelName": "deepseek-r1"},
                            {"modelName": "gemini-3.1-pro-preview"},
                        ],
                        api_base_url="https://aitoplus.com",
                        access_token="fresh-token",
                        default_chat_model="gemini-3.1-pro-preview",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            data = json.loads(config_path.read_text(encoding="utf-8"))
            defaults = data["agents"]["defaults"]["models"]

            self.assertEqual(defaults["omnidrive/gemini-3.1-pro-preview"]["alias"], "omni")
            self.assertEqual(defaults["omnidrive/claude-opus-4-6-thinking"]["alias"], "omni-claude-opus-4-6-thinking")
            self.assertEqual(defaults["omnidrive/deepseek-chat"]["alias"], "omni-deepseek-chat")
            self.assertEqual(defaults["omnidrive/deepseek-r1"]["alias"], "omni-deepseek-r1")

    def test_sync_openclaw_configs_bootstraps_missing_provider_and_agent_models_file(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            models_path = Path(temp_dir) / "agents" / "main" / "agent" / "models.json"
            config_path.write_text(
                json.dumps(
                    {
                        "agents": {
                            "defaults": {
                                "workspace": "/tmp/openclaw-workspace",
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path, models_path)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 5409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", ""):
                    sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "deepseek-chat"}, {"modelName": "gpt-5.4"}],
                        api_base_url="https://aitoplus.com",
                        access_token="fresh-token",
                        default_chat_model="deepseek-chat",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            root_config = json.loads(config_path.read_text(encoding="utf-8"))
            agent_models = json.loads(models_path.read_text(encoding="utf-8"))

            self.assertEqual(root_config["models"]["providers"]["omnidrive"]["baseUrl"], "http://127.0.0.1:5409/openai/v1")
            self.assertEqual(root_config["models"]["providers"]["omnidrive"]["api"], "openai-completions")
            self.assertNotIn("apiKey", root_config["models"]["providers"]["omnidrive"])
            self.assertEqual(
                [item["id"] for item in root_config["models"]["providers"]["omnidrive"]["models"]],
                ["deepseek-chat", "gpt-5.4"],
            )
            self.assertEqual(root_config["agents"]["defaults"]["workspace"], "/tmp/openclaw-workspace")
            self.assertEqual(root_config["agents"]["defaults"]["models"]["omnidrive/deepseek-chat"]["alias"], "omni")
            self.assertEqual(root_config["agents"]["defaults"]["models"]["omnidrive/gpt-5.4"]["alias"], "omni-gpt")
            self.assertTrue(models_path.exists())
            self.assertNotIn("apiKey", agent_models["providers"]["omnidrive"])
            self.assertEqual(
                [item["id"] for item in agent_models["providers"]["omnidrive"]["models"]],
                ["deepseek-chat", "gpt-5.4"],
            )

    def test_sync_openclaw_configs_skips_writes_when_serialized_content_unchanged(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            models_path = Path(temp_dir) / "agents" / "main" / "agent" / "models.json"
            config_path.write_text(
                json.dumps(
                    {
                        "agents": {
                            "defaults": {
                                "workspace": "/tmp/openclaw-workspace",
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path, models_path)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 5409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", ""):
                    first_changed_paths = sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "deepseek-chat"}, {"modelName": "gpt-5.4"}],
                        api_base_url="https://aitoplus.com",
                        access_token="fresh-token",
                        default_chat_model="deepseek-chat",
                    )
                    second_changed_paths = sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "deepseek-chat"}, {"modelName": "gpt-5.4"}],
                        api_base_url="https://aitoplus.com",
                        access_token="fresh-token",
                        default_chat_model="deepseek-chat",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            self.assertEqual(
                first_changed_paths,
                [str(config_path), str(models_path)],
            )
            self.assertEqual(second_changed_paths, [])

    def test_sync_openclaw_configs_uses_local_gateway_key_when_present(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            config_path = Path(temp_dir) / "openclaw.json"
            config_path.write_text(
                json.dumps({"models": {"providers": {}}, "agents": {"defaults": {"models": {}}}}),
                encoding="utf-8",
            )

            original_paths = sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS
            try:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (config_path,)
                with mock.patch.object(sau_backend, "SAU_BACKEND_PORT", 15409), \
                     mock.patch.object(sau_backend, "OMNIBULL_API_KEY", "local-gateway-key"):
                    sau_backend.sync_openclaw_omnidrive_model_configs(
                        [{"modelName": "gpt-5.4"}],
                        api_base_url="https://cloud.example.com",
                        access_token="cloud-access-token",
                        default_chat_model="gpt-5.4",
                    )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            provider = json.loads(config_path.read_text(encoding="utf-8"))["models"]["providers"]["omnidrive"]
            self.assertEqual(provider["baseUrl"], "http://127.0.0.1:15409/openai/v1")
            self.assertEqual(provider["apiKey"], "local-gateway-key")

    def test_ensure_openclaw_omnidrive_models_synced_only_logs_changed_paths(self):
        with mock.patch.object(
            sau_backend,
            "sync_openclaw_omnidrive_models_from_cloud",
            return_value=["/tmp/openclaw.json", "/tmp/models.json"],
        ), mock.patch.object(
            sau_backend,
            "_reload_openclaw_gateway_after_omnidrive_model_sync",
        ) as reload_mock:
            sau_backend.ensure_openclaw_omnidrive_models_synced()

        reload_mock.assert_not_called()

    def test_ensure_openclaw_omnidrive_models_synced_skips_reload_when_configs_unchanged(self):
        with mock.patch.object(
            sau_backend,
            "sync_openclaw_omnidrive_models_from_cloud",
            return_value=[],
        ), mock.patch.object(
            sau_backend,
            "_reload_openclaw_gateway_after_omnidrive_model_sync",
        ) as reload_mock:
            sau_backend.ensure_openclaw_omnidrive_models_synced()

        reload_mock.assert_not_called()

    def test_refresh_openclaw_runtime_config_reloads_gateway_when_model_configs_change(self):
        session_payload = {
            "accessToken": "session-token",
            "apiBaseUrl": "https://aitoplus.com",
            "device": {"defaultChatModel": "gpt-5.4"},
        }
        cloud_payload = {
            "items": [
                {"modelName": "gpt-5.4"},
                {"modelName": "deepseek-chat"},
            ]
        }

        with mock.patch.object(
            sau_backend,
            "get_omnidrive_device_session_data",
            return_value=session_payload,
        ), mock.patch.object(
            sau_backend,
            "omnidrive_cloud_json_request",
            return_value=(200, cloud_payload),
        ), mock.patch.object(
            sau_backend,
            "_extract_omnidrive_chat_models",
            return_value=cloud_payload["items"],
        ), mock.patch.object(
            sau_backend,
            "_mark_openclaw_omnidrive_model_sync",
        ), mock.patch.object(
            sau_backend,
            "sync_openclaw_omnidrive_model_configs",
            return_value=["/tmp/openclaw.json"],
        ), mock.patch.object(
            sau_backend,
            "sync_hermes_shared_runtime_config",
            return_value=({}, []),
        ), mock.patch.object(
            sau_backend,
            "_reload_openclaw_gateway_after_omnidrive_model_sync",
        ) as reload_mock:
            sau_backend.refresh_openclaw_omnidrive_runtime_config(include_models=True)

        reload_mock.assert_called_once_with(
            ["/tmp/openclaw.json"],
            reason="refresh_openclaw_omnidrive_runtime_config",
        )

    def test_refresh_openclaw_runtime_config_skips_reload_when_model_configs_unchanged(self):
        session_payload = {
            "accessToken": "session-token",
            "apiBaseUrl": "https://aitoplus.com",
            "device": {"defaultChatModel": "gpt-5.4"},
        }
        cloud_payload = {
            "items": [
                {"modelName": "gpt-5.4"},
            ]
        }

        with mock.patch.object(
            sau_backend,
            "get_omnidrive_device_session_data",
            return_value=session_payload,
        ), mock.patch.object(
            sau_backend,
            "omnidrive_cloud_json_request",
            return_value=(200, cloud_payload),
        ), mock.patch.object(
            sau_backend,
            "_extract_omnidrive_chat_models",
            return_value=cloud_payload["items"],
        ), mock.patch.object(
            sau_backend,
            "_mark_openclaw_omnidrive_model_sync",
        ), mock.patch.object(
            sau_backend,
            "sync_openclaw_omnidrive_model_configs",
            return_value=[],
        ), mock.patch.object(
            sau_backend,
            "sync_hermes_shared_runtime_config",
            return_value=({}, []),
        ), mock.patch.object(
            sau_backend,
            "_reload_openclaw_gateway_after_omnidrive_model_sync",
        ) as reload_mock:
            sau_backend.refresh_openclaw_omnidrive_runtime_config(include_models=True)

        reload_mock.assert_not_called()

    def test_skill_routes_allow_loopback_without_omnibull_key_and_reject_remote_without_key(self):
        with mock.patch.object(sau_backend, "OMNIBULL_API_KEY", "local-gateway-key"), \
             mock.patch.object(sau_backend, "build_skill_status_payload", return_value={"deviceCode": "device-1"}):
            with sau_backend.app.test_client() as client:
                local_response = client.get("/api/skill/status", environ_base={"REMOTE_ADDR": "127.0.0.1"})
                remote_response = client.get("/api/skill/status", environ_base={"REMOTE_ADDR": "10.20.30.40"})

        self.assertEqual(local_response.status_code, 200)
        self.assertEqual(remote_response.status_code, 401)

    def test_skill_routes_do_not_trust_forwarded_for_when_loopback_key_is_required(self):
        with mock.patch.object(sau_backend, "OMNIBULL_API_KEY", "local-gateway-key"), \
             mock.patch.object(sau_backend, "build_skill_status_payload", return_value={"deviceCode": "device-1"}):
            with sau_backend.app.test_client() as client:
                response = client.get(
                    "/api/skill/status",
                    environ_base={"REMOTE_ADDR": "10.20.30.40"},
                    headers={"X-Forwarded-For": "127.0.0.1"},
                )

        self.assertEqual(response.status_code, 401)

    def test_skill_omnidrive_session_returns_auth_state_summary(self):
        session_payload = {
            "accessToken": "session-token",
            "expiresAt": "2026-04-19T10:00:00Z",
            "apiBaseUrl": "https://cloud.example.com",
            "cloudUrl": "https://cloud.example.com",
            "user": {"id": "user-1", "name": "禾硕AI", "email": "demo@example.com"},
            "device": {"id": "device-1", "deviceCode": "device-code-1", "name": "Factory OmniBull"},
        }

        with mock.patch.object(sau_backend, "OMNIBULL_API_KEY", "local-gateway-key"), \
             mock.patch.object(sau_backend, "fetch_omnidrive_device_session", return_value=(200, session_payload)), \
             mock.patch.object(sau_backend, "sync_openclaw_omnidrive_model_configs"), \
             mock.patch.object(sau_backend, "sync_hermes_shared_runtime_config"):
            with sau_backend.app.test_client() as client:
                response = client.get("/api/skill/omnidrive/session", environ_base={"REMOTE_ADDR": "127.0.0.1"})

        self.assertEqual(response.status_code, 200)
        data = response.get_json()["data"]
        self.assertEqual(data["authState"], "authorized")
        self.assertEqual(data["reason"], "")
        self.assertEqual(data["availableSkills"], sau_backend.OPENCLAW_OMNIDRIVE_AVAILABLE_SKILLS)

    def test_summarize_openai_media_tool_result_includes_urls_and_artifacts(self):
        content = json.dumps(
            {
                "job": {
                    "id": "job-image-1",
                    "jobType": "image",
                    "modelName": "gemini-3-pro-image-preview",
                    "status": "success",
                },
                "workspace": {
                    "jobId": "job-image-1",
                    "jobType": "image",
                    "modelName": "gemini-3-pro-image-preview",
                    "status": "success",
                    "publicUrls": ["https://cdn.example.com/result.png"],
                    "artifacts": [
                        {
                            "artifactType": "image",
                            "fileName": "result.png",
                            "mimeType": "image/png",
                            "publicUrl": "https://cdn.example.com/result.png",
                        }
                    ],
                },
            },
            ensure_ascii=False,
        )

        summary = sau_backend._summarize_openai_media_tool_result("omnidrive_image", content)

        self.assertIn("图片已生成完成。", summary)
        self.assertIn("任务 ID：job-image-1", summary)
        self.assertIn("https://cdn.example.com/result.png", summary)
        self.assertIn("result.png (image/png)", summary)

    def test_delete_account_notifies_omnidrive_with_deleted_status(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            db_dir = base_dir / "db"
            db_dir.mkdir(parents=True, exist_ok=True)
            db_path = db_dir / "database.db"

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
                    (4, "kuaishou-cookie.json", "测试快手账号", 1),
                )
                conn.commit()

            request_calls = []

            class DummyOmniDriveAgent:
                device_code = "device-test"

                def _request(self, method, path, *, params=None, payload=None):
                    request_calls.append((method, path, payload))
                    return {"deleted": True}

            original_base_dir = sau_backend.BASE_DIR
            original_agent = sau_backend.omnidrive_agent
            try:
                sau_backend.BASE_DIR = base_dir
                sau_backend.omnidrive_agent = DummyOmniDriveAgent()
                with mock.patch.object(sau_backend, "clear_account_storage_state", autospec=True):
                    with sau_backend.app.test_client() as client:
                        response = client.get("/deleteAccount?id=1")
            finally:
                sau_backend.BASE_DIR = original_base_dir
                sau_backend.omnidrive_agent = original_agent

            self.assertEqual(response.status_code, 200)
            self.assertEqual(
                request_calls,
                [
                    (
                        "POST",
                        "/api/v1/agent/accounts/sync",
                        {
                            "deviceCode": "device-test",
                            "platform": "快手",
                            "accountName": "测试快手账号",
                            "status": "deleted",
                            "lastMessage": "Account explicitly deleted by user locally",
                        },
                    )
                ],
            )
            with sqlite3.connect(db_path) as conn:
                cursor = conn.cursor()
                cursor.execute("SELECT COUNT(*) FROM user_info")
                remaining = cursor.fetchone()[0]
            self.assertEqual(remaining, 0)


class OpenClawGatewayDailyReloadTests(unittest.TestCase):
    def test_compute_next_openclaw_gateway_daily_reload_epoch_rolls_to_next_day_after_cutoff(self):
        with mock.patch.object(sau_backend, "OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR", 0), \
             mock.patch.object(sau_backend, "OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE", 0):
            before_midnight = datetime(2026, 4, 16, 23, 59, 30)
            after_midnight = datetime(2026, 4, 17, 0, 0, 1)

            next_before = datetime.fromtimestamp(
                sau_backend._compute_next_openclaw_gateway_daily_reload_epoch(before_midnight)
            )
            next_after = datetime.fromtimestamp(
                sau_backend._compute_next_openclaw_gateway_daily_reload_epoch(after_midnight)
            )

        self.assertEqual(next_before, datetime(2026, 4, 17, 0, 0, 0))
        self.assertEqual(next_after, datetime(2026, 4, 18, 0, 0, 0))

    def test_attempt_openclaw_gateway_daily_reload_defers_when_publish_task_running(self):
        with mock.patch.object(sau_backend, "OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS", 300), \
             mock.patch.object(sau_backend, "ensure_publish_task_manager_started"), \
             mock.patch.object(
                 sau_backend.publish_task_manager,
                 "list_tasks",
                 return_value=[{"taskUuid": "task-running-1", "status": "running"}],
             ), \
             mock.patch.object(sau_backend, "_restart_openclaw_gateway_for_config_reload") as restart_mock:
            result = sau_backend._attempt_openclaw_gateway_daily_reload()

        self.assertEqual(result["status"], "deferred")
        self.assertEqual(result["delaySeconds"], 300)
        self.assertEqual(result["runningTaskCount"], 1)
        self.assertEqual(result["taskUuids"], ["task-running-1"])
        restart_mock.assert_not_called()

    def test_attempt_openclaw_gateway_daily_reload_restarts_when_idle(self):
        with mock.patch.object(sau_backend, "ensure_publish_task_manager_started"), \
             mock.patch.object(sau_backend.publish_task_manager, "list_tasks", return_value=[]), \
             mock.patch.object(
                 sau_backend,
                 "_restart_openclaw_gateway_for_config_reload",
                 return_value={"ok": True, "command": "openclaw gateway restart"},
             ) as restart_mock:
            result = sau_backend._attempt_openclaw_gateway_daily_reload()

        self.assertEqual(result["status"], "restarted")
        self.assertEqual(result["command"], "openclaw gateway restart")
        restart_mock.assert_called_once()

    def test_attempt_openclaw_gateway_daily_reload_retries_when_restart_fails(self):
        with mock.patch.object(sau_backend, "OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS", 600), \
             mock.patch.object(sau_backend, "ensure_publish_task_manager_started"), \
             mock.patch.object(sau_backend.publish_task_manager, "list_tasks", return_value=[]), \
             mock.patch.object(
                 sau_backend,
                 "_restart_openclaw_gateway_for_config_reload",
                 return_value={
                     "ok": False,
                     "command": "openclaw gateway restart",
                     "returncode": 1,
                     "error": "",
                     "stdout": "warn",
                     "stderr": "boom",
                 },
             ):
            result = sau_backend._attempt_openclaw_gateway_daily_reload()

        self.assertEqual(result["status"], "retry")
        self.assertEqual(result["delaySeconds"], 600)
        self.assertEqual(result["returncode"], 1)
        self.assertEqual(result["stderr"], "boom")


class OpenBackendRouteTests(unittest.TestCase):
    def setUp(self):
        sau_backend.active_backend_sessions.clear()

    def tearDown(self):
        sau_backend.active_backend_sessions.clear()

    def _create_user_info_db(self, db_path, rows=None):
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
            for row in rows or []:
                cursor.execute(
                    """
                    INSERT INTO user_info (type, filePath, userName, status)
                    VALUES (?, ?, ?, ?)
                    """,
                    row,
                )
            conn.commit()

    def test_open_backend_route_returns_404_when_account_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            db_dir = base_dir / "db"
            db_dir.mkdir(parents=True, exist_ok=True)
            self._create_user_info_db(db_dir / "database.db")

            original_base_dir = sau_backend.BASE_DIR
            try:
                sau_backend.BASE_DIR = base_dir
                with sau_backend.app.test_client() as client:
                    response = client.post("/api/accounts/99/open-backend")
            finally:
                sau_backend.BASE_DIR = original_base_dir

        self.assertEqual(response.status_code, 404)
        self.assertEqual(response.get_json()["msg"], "账号不存在")

    def test_open_backend_route_rejects_unsupported_platform(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            db_dir = base_dir / "db"
            db_dir.mkdir(parents=True, exist_ok=True)
            self._create_user_info_db(
                db_dir / "database.db",
                rows=[(1, "xhs-cookie.json", "测试小红书账号", 1)],
            )

            original_base_dir = sau_backend.BASE_DIR
            try:
                sau_backend.BASE_DIR = base_dir
                with sau_backend.app.test_client() as client:
                    response = client.post("/api/accounts/1/open-backend")
            finally:
                sau_backend.BASE_DIR = original_base_dir

        self.assertEqual(response.status_code, 400)
        self.assertIn("仅支持抖音、视频号、快手", response.get_json()["msg"])

    def test_open_backend_route_starts_supported_session(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            db_dir = base_dir / "db"
            db_dir.mkdir(parents=True, exist_ok=True)
            self._create_user_info_db(
                db_dir / "database.db",
                rows=[(3, "douyin-cookie.json", "测试抖音账号", 1)],
            )

            original_base_dir = sau_backend.BASE_DIR
            try:
                sau_backend.BASE_DIR = base_dir
                with mock.patch.object(
                    sau_backend,
                    "get_active_backend_session",
                    return_value=None,
                ), mock.patch.object(
                    sau_backend,
                    "start_backend_session",
                    return_value=(
                        {
                            "accountId": 1,
                            "platformType": 3,
                            "accountName": "测试抖音账号",
                            "startedAt": "2026-04-08T00:00:00+00:00",
                        },
                        None,
                    ),
                ):
                    with sau_backend.app.test_client() as client:
                        response = client.post("/api/accounts/1/open-backend")
            finally:
                sau_backend.BASE_DIR = original_base_dir

        payload = response.get_json()
        self.assertEqual(response.status_code, 200)
        self.assertEqual(payload["msg"], "已在本机打开后台")
        self.assertEqual(payload["data"]["platformType"], 3)

    def test_open_backend_route_rejects_duplicate_session(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            db_dir = base_dir / "db"
            db_dir.mkdir(parents=True, exist_ok=True)
            self._create_user_info_db(
                db_dir / "database.db",
                rows=[(4, "kuaishou-cookie.json", "测试快手账号", 1)],
            )

            original_base_dir = sau_backend.BASE_DIR
            try:
                sau_backend.BASE_DIR = base_dir
                with mock.patch.object(
                    sau_backend,
                    "get_active_backend_session",
                    return_value={
                        "accountId": 1,
                        "platformType": 4,
                        "accountName": "测试快手账号",
                        "startedAt": "2026-04-08T00:00:00+00:00",
                    },
                ):
                    with sau_backend.app.test_client() as client:
                        response = client.post("/api/accounts/1/open-backend")
            finally:
                sau_backend.BASE_DIR = original_base_dir

        payload = response.get_json()
        self.assertEqual(response.status_code, 409)
        self.assertEqual(payload["msg"], "该账号后台已打开，请勿重复启动")


class LoginSseStreamTests(unittest.TestCase):
    def test_sse_stream_maps_login_failed_payload_to_login_failed_event(self):
        status_queue = Queue()
        status_queue.put(
            {
                "type": "login_failed",
                "payload": {
                    "message": "登录后未能确认本地登录态已生效",
                },
            }
        )

        stream = sau_backend.sse_stream(status_queue)
        self.assertTrue(next(stream).startswith(": "))
        chunk = next(stream)

        self.assertEqual(
            chunk,
            "event: login_failed\ndata: 登录后未能确认本地登录态已生效\n\n",
        )

    def test_sse_stream_keeps_plain_done_event_for_success(self):
        status_queue = Queue()
        status_queue.put("200")

        stream = sau_backend.sse_stream(status_queue)
        self.assertTrue(next(stream).startswith(": "))
        chunk = next(stream)

        self.assertEqual(chunk, "event: done\ndata: 200\n\n")

    def test_sse_stream_maps_cancelled_to_cancelled_event(self):
        status_queue = Queue()
        status_queue.put("CANCELLED")

        stream = sau_backend.sse_stream(status_queue)
        self.assertTrue(next(stream).startswith(": "))
        chunk = next(stream)

        self.assertEqual(
            chunk,
            "event: cancelled\ndata: 本地登录浏览器已关闭，本次添加账号未完成\n\n",
        )


if __name__ == "__main__":
    unittest.main()
