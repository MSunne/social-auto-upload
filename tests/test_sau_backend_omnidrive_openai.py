import os
import unittest

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


if __name__ == "__main__":
    unittest.main()
