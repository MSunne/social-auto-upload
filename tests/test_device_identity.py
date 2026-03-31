import json
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

from utils import device_identity


def build_conf(device_code):
    return SimpleNamespace(
        OMNIBULL_DEVICE_IDENTITY_FILE="",
        OMNIBULL_DEVICE_CODE=device_code,
        OMNIDRIVE_DEVICE_CODE="",
        OMNIDRIVE_AGENT_KEY="",
        OMNIBULL_DEVICE_NAME="A001",
        OMNIDRIVE_DEVICE_NAME="",
        OMNIBULL_API_KEY="",
    )


class DeviceIdentityTests(unittest.TestCase):
    def test_existing_auto_identity_without_fingerprint_keeps_stored_device_code(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            identity_path = base_dir / "runtime" / "device.identity.json"
            identity_path.parent.mkdir(parents=True, exist_ok=True)
            identity_path.write_text(
                json.dumps(
                    {
                        "identityVersion": 1,
                        "provisionMethod": "auto_bootstrap",
                        "provisionedAt": "2026-03-01T00:00:00+00:00",
                        "deviceCode": "5e:f9:01:22:73:09",
                        "agentKey": "keep-me",
                        "deviceName": "A001",
                        "localApiKey": "local-key",
                    }
                ),
                encoding="utf-8",
            )

            with mock.patch.object(device_identity, "get_device_fingerprint", return_value="same-machine"):
                payload = device_identity.load_device_identity(build_conf("e2:c5:a0:19:91:11"), str(base_dir))

            self.assertEqual(payload["source"], "device_identity_file_updated")
            self.assertEqual(payload["deviceCode"], "5e:f9:01:22:73:09")
            self.assertEqual(payload["agentKey"], "keep-me")
            self.assertEqual(payload["localApiKey"], "local-key")
            stored = json.loads(identity_path.read_text(encoding="utf-8"))
            self.assertEqual(stored["deviceCode"], "5e:f9:01:22:73:09")
            self.assertEqual(stored["deviceFingerprint"], "same-machine")

    def test_auto_identity_rotates_when_machine_fingerprint_changes(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_dir = Path(temp_dir)
            identity_path = base_dir / "runtime" / "device.identity.json"
            identity_path.parent.mkdir(parents=True, exist_ok=True)
            identity_path.write_text(
                json.dumps(
                    {
                        "identityVersion": 2,
                        "provisionMethod": "auto_bootstrap",
                        "provisionedAt": "2026-03-01T00:00:00+00:00",
                        "deviceCode": "5e:f9:01:22:73:09",
                        "agentKey": "old-agent",
                        "deviceName": "A001",
                        "localApiKey": "local-key",
                        "deviceFingerprint": "old-machine",
                    }
                ),
                encoding="utf-8",
            )

            with mock.patch.object(device_identity, "get_device_fingerprint", return_value="new-machine"):
                payload = device_identity.load_device_identity(build_conf("00:e0:b4:69:4a:df"), str(base_dir))

            self.assertEqual(payload["source"], "device_identity_file_rotated")
            self.assertEqual(payload["deviceCode"], "00:e0:b4:69:4a:df")
            self.assertEqual(payload["deviceFingerprint"], "new-machine")
            self.assertNotEqual(payload["agentKey"], "old-agent")
            self.assertEqual(payload["localApiKey"], "local-key")


if __name__ == "__main__":
    unittest.main()
