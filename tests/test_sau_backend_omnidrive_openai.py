import os
import json
import tempfile
import unittest
import sqlite3
from pathlib import Path
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
                sau_backend.sync_openclaw_omnidrive_model_configs(
                    [{"modelName": "gpt-5.4"}],
                    api_base_url="http://127.0.0.1:8410",
                    access_token="fresh-token",
                    default_chat_model="gpt-5.4",
                )
            finally:
                sau_backend.OPENCLAW_OMNIDRIVE_CONFIG_PATHS = original_paths

            data = json.loads(config_path.read_text(encoding="utf-8"))
            defaults = data["agents"]["defaults"]["models"]

            self.assertNotIn("omnidrive/gpt-5-4", defaults)
            self.assertNotIn("omnidrive/default-chat", defaults)
            self.assertEqual(defaults["omnidrive/gpt-5.4"]["alias"], "omni-gpt")
            self.assertEqual(data["models"]["providers"]["omnidrive"]["apiKey"], "fresh-token")

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


if __name__ == "__main__":
    unittest.main()
