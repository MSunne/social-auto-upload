import importlib.util
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "scripts" / "seed_omnidrive_mock_data.py"


def load_seed_module():
    spec = importlib.util.spec_from_file_location("seed_omnidrive_mock_data", SCRIPT_PATH)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class OmniDriveDemoSeedTests(unittest.TestCase):
    def test_demo_user_defaults_target_heshuo_ai(self):
        module = load_seed_module()

        self.assertEqual(module.DEMO_USER["phone"], "18812345678")
        self.assertEqual(module.DEMO_USER["name"], "禾硕AI")
        self.assertEqual(module.DEMO_USER["password"], "123456")

    def test_demo_seed_definitions_match_expected_scale(self):
        module = load_seed_module()

        accounts = module.build_demo_account_specs()
        image_jobs = module.build_demo_image_job_specs()
        video_jobs = module.build_demo_video_job_specs()
        publish_tasks = module.build_demo_publish_task_specs()
        login_sessions = module.build_demo_login_session_specs()
        platforms = {item["platform"] for item in accounts}

        self.assertEqual(len(accounts), 24)
        self.assertEqual(len(platforms), 10)
        self.assertTrue({"Instagram", "Facebook", "YouTube"}.issubset(platforms))
        self.assertEqual(len(image_jobs), 30)
        self.assertEqual(len(video_jobs), 30)
        self.assertEqual(len(publish_tasks), 24)
        self.assertEqual(len(login_sessions), 10)

        self.assertEqual(sum(1 for item in image_jobs if item["status"] in {"success", "completed"}), 27)
        self.assertEqual(sum(1 for item in video_jobs if item["status"] in {"success", "completed"}), 27)
        self.assertEqual(sum(1 for item in publish_tasks if item["status"] == "success"), 21)
        self.assertLessEqual(sum(1 for item in accounts if item["status"] == "waiting_verify"), 1)


if __name__ == "__main__":
    unittest.main()
