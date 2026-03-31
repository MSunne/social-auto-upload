import unittest
from unittest import mock

from utils import device_meta


class DeviceMetaTests(unittest.TestCase):
    def test_selects_active_physical_interface_with_ipv4(self):
        candidates = [
            {
                "interface": "enp1s0",
                "mac": "00:e0:b4:69:4a:de",
                "is_up": False,
                "physical": True,
                "score": 120,
            },
            {
                "interface": "enp2s0",
                "mac": "00:e0:b4:69:4a:df",
                "is_up": True,
                "physical": True,
                "score": 170,
            },
        ]
        with mock.patch.object(device_meta, "_iter_psutil_mac_candidates", return_value=candidates):
            candidate = device_meta._select_device_mac_candidate()

        self.assertIsNotNone(candidate)
        self.assertEqual(candidate["mac"], "00:e0:b4:69:4a:df")

    def test_get_device_code_falls_back_to_uuid_when_no_interface_candidate(self):
        with mock.patch.object(device_meta, "_iter_psutil_mac_candidates", return_value=[]), mock.patch.object(
            device_meta, "_iter_linux_sysfs_mac_candidates", return_value=[]
        ), mock.patch("utils.device_meta.uuid.getnode", return_value=0x001122334455):
            self.assertEqual(device_meta.get_device_code(), "00:11:22:33:44:55")


if __name__ == "__main__":
    unittest.main()
