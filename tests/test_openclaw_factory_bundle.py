import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


def load_bundle_module():
    module_path = (
        Path(__file__).resolve().parents[1] / "scripts" / "openclaw_factory_bundle.py"
    )
    spec = importlib.util.spec_from_file_location("openclaw_factory_bundle", module_path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class OpenClawFactoryBundleTests(unittest.TestCase):
    def test_build_deploy_manifest_rewrites_plugins_and_skills(self):
        module = load_bundle_module()
        source_config = {
            "plugins": {
                "entries": {
                    "feishu": {"enabled": True},
                    "qwen-portal-auth": {"enabled": True},
                    "wecom-openclaw-plugin": {"enabled": False},
                    "omnibull": {"enabled": True},
                    "omnidrive": {
                        "enabled": True,
                        "config": {"localDeviceCode": "device-1", "other": "keep-me"},
                    },
                    "openclaw-weixin": {"enabled": True},
                },
                "installs": {
                    "feishu": {"resolvedSpec": "@openclaw/feishu@2026.3.7"},
                    "wecom-openclaw-plugin": {
                        "resolvedSpec": "@wecom/wecom-openclaw-plugin@1.0.9"
                    },
                },
            },
            "skills": {
                "entries": {
                    "feishu-doc": {"enabled": True},
                    "feishu-drive": {"enabled": True},
                    "disabled-skill": {"enabled": False},
                }
            },
        }

        manifest = module.build_deploy_manifest(
            source_config=source_config,
            app_root="/opt/omnibull/social-auto-upload",
            omnidrive_base_url="https://aitoplus.com",
        )

        self.assertEqual(
            manifest["plugins"]["allow"],
            [
                "feishu",
                "wecom-openclaw-plugin",
                "omnibull",
                "omnidrive",
                "openclaw-weixin",
            ],
        )
        self.assertNotIn("qwen-portal-auth", manifest["plugins"]["entries"])
        self.assertEqual(
            manifest["plugins"]["entries"]["omnibull"]["config"]["baseUrl"],
            "http://127.0.0.1:5409",
        )
        self.assertEqual(
            manifest["plugins"]["entries"]["omnidrive"]["config"],
            {
                "baseUrl": "https://aitoplus.com",
                "localOmniBullBaseUrl": "http://127.0.0.1:5409",
                "timeoutMs": 45000,
            },
        )
        self.assertEqual(manifest["plugins"]["load"]["paths"], [])
        self.assertEqual(
            manifest["skills"]["entries"],
            {
                "feishu-doc": {"enabled": True},
                "feishu-drive": {"enabled": True},
            },
        )
        self.assertEqual(
            manifest["plugins"]["install_specs"],
            {
                "feishu": "@openclaw/feishu@2026.3.7",
                "wecom-openclaw-plugin": "@wecom/wecom-openclaw-plugin@1.0.9",
            },
        )

    def test_merge_remote_config_rewrites_gateway_for_loopback_proxy(self):
        module = load_bundle_module()
        manifest = {
            "plugins": {
                "allow": ["omnibull", "omnidrive", "feishu", "openclaw-weixin"],
                "load": {"paths": []},
                "entries": {
                    "omnibull": {
                        "enabled": True,
                        "config": {"baseUrl": "http://127.0.0.1:5409", "timeoutMs": 15000},
                    },
                    "omnidrive": {
                        "enabled": True,
                        "config": {
                            "baseUrl": "https://aitoplus.com",
                            "localOmniBullBaseUrl": "http://127.0.0.1:5409",
                            "timeoutMs": 45000,
                        },
                    },
                    "feishu": {"enabled": True},
                    "openclaw-weixin": {"enabled": True},
                    "wecom-openclaw-plugin": {"enabled": False},
                },
            },
            "skills": {"entries": {"feishu-doc": {"enabled": True}}},
        }
        existing = {
            "gateway": {
                "mode": "cloud",
                "bind": "0.0.0.0",
                "port": 9999,
                "auth": {"mode": "token", "token": "keep-this-token"},
                "nodes": {"denyCommands": ["camera.snap"]},
            },
            "plugins": {"entries": {"qwen-portal-auth": {"enabled": True}}},
            "skills": {"entries": {"old-skill": {"enabled": True}}},
        }

        merged = module.merge_remote_config(existing, manifest)

        self.assertEqual(merged["gateway"]["mode"], "local")
        self.assertEqual(merged["gateway"]["bind"], "loopback")
        self.assertEqual(merged["gateway"]["port"], 18790)
        self.assertEqual(merged["gateway"]["auth"]["mode"], "none")
        self.assertNotIn("token", merged["gateway"]["auth"])
        self.assertEqual(
            merged["gateway"]["controlUi"]["allowedOrigins"],
            ["http://192.168.1.24:18789"],
        )
        self.assertTrue(
            merged["gateway"]["controlUi"]["dangerouslyDisableDeviceAuth"]
        )
        self.assertEqual(merged["gateway"]["nodes"], {"denyCommands": ["camera.snap"]})
        self.assertEqual(merged["plugins"]["load"]["paths"], [])
        self.assertEqual(
            merged["plugins"]["entries"]["omnidrive"]["config"]["baseUrl"],
            "https://aitoplus.com",
        )
        self.assertNotIn("localDeviceCode", merged["plugins"]["entries"]["omnidrive"]["config"])
        self.assertNotIn("qwen-portal-auth", merged["plugins"]["entries"])
        self.assertEqual(merged["skills"]["entries"]["feishu-doc"], {"enabled": True})
        self.assertEqual(merged["skills"]["entries"]["old-skill"], {"enabled": True})

    def test_export_bundle_copies_selected_extensions_and_runtime_files(self):
        module = load_bundle_module()
        with tempfile.TemporaryDirectory() as temp_dir:
            temp_path = Path(temp_dir)
            source_home = temp_path / "source-home"
            extensions_dir = source_home / "extensions"
            (extensions_dir / "omnidrive").mkdir(parents=True)
            (extensions_dir / "omnidrive" / "package.json").write_text(
                '{"name":"omnidrive"}', encoding="utf-8"
            )
            (extensions_dir / "openclaw-weixin").mkdir(parents=True)
            (extensions_dir / "openclaw-weixin" / "package.json").write_text(
                '{"name":"openclaw-weixin"}', encoding="utf-8"
            )
            (extensions_dir / "feishu").mkdir(parents=True)
            (extensions_dir / "feishu" / "package.json").write_text(
                '{"name":"feishu"}', encoding="utf-8"
            )
            (extensions_dir / "wecom-openclaw-plugin").mkdir(parents=True)
            (extensions_dir / "wecom-openclaw-plugin" / "package.json").write_text(
                '{"name":"wecom"}', encoding="utf-8"
            )
            (source_home / "feishu").mkdir()
            (source_home / "feishu" / "dedup.json").write_text("{}", encoding="utf-8")
            (source_home / "openclaw-weixin").mkdir()
            (source_home / "openclaw-weixin" / "accounts.json").write_text(
                "{}", encoding="utf-8"
            )
            (source_home / "wecom").mkdir()
            (source_home / "wecom" / "reqid.json").write_text("{}", encoding="utf-8")
            (source_home / "wecomConfig").mkdir()
            (source_home / "wecomConfig" / "config.json").write_text(
                "{}", encoding="utf-8"
            )
            (source_home / "credentials").mkdir()
            (source_home / "credentials" / "feishu-main-allowFrom.json").write_text(
                "{}", encoding="utf-8"
            )
            (source_home / "openclaw.json").write_text(
                json.dumps(
                    {
                        "plugins": {
                            "entries": {
                                "feishu": {"enabled": True},
                                "wecom-openclaw-plugin": {"enabled": False},
                                "omnibull": {"enabled": True},
                                "omnidrive": {"enabled": True},
                                "openclaw-weixin": {"enabled": True},
                            },
                            "installs": {
                                "feishu": {"resolvedSpec": "@openclaw/feishu@2026.3.7"}
                            },
                        },
                        "skills": {"entries": {"feishu-doc": {"enabled": True}}},
                    }
                ),
                encoding="utf-8",
            )

            output_dir = temp_path / "bundle"
            manifest = module.export_bundle(
                source_home=source_home,
                output_dir=output_dir,
                app_root="/opt/omnibull/social-auto-upload",
                omnidrive_base_url="https://aitoplus.com",
            )

            self.assertTrue((output_dir / "extensions" / "omnibull" / "package.json").exists())
            self.assertTrue((output_dir / "extensions" / "omnidrive" / "package.json").exists())
            self.assertTrue((output_dir / "extensions" / "openclaw-weixin" / "package.json").exists())
            self.assertTrue((output_dir / "extensions" / "feishu" / "package.json").exists())
            self.assertTrue(
                (output_dir / "extensions" / "wecom-openclaw-plugin" / "package.json").exists()
            )
            self.assertTrue((output_dir / "state" / "feishu" / "dedup.json").exists())
            self.assertTrue((output_dir / "state" / "openclaw-weixin" / "accounts.json").exists())
            self.assertTrue((output_dir / "state" / "wecom" / "reqid.json").exists())
            self.assertTrue((output_dir / "state" / "wecomConfig" / "config.json").exists())
            self.assertTrue(
                (
                    output_dir
                    / "state"
                    / "credentials"
                    / "feishu-main-allowFrom.json"
                ).exists()
            )
            self.assertEqual(
                manifest["plugins"]["entries"]["wecom-openclaw-plugin"]["enabled"], False
            )


if __name__ == "__main__":
    unittest.main()
