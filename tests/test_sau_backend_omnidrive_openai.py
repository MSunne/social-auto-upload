import os
import json
import tempfile
import unittest
from pathlib import Path

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


if __name__ == "__main__":
    unittest.main()
