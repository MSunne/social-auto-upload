import sqlite3
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from utils.platform_capabilities import (
    cache_platform_capabilities_from_session_payload,
    ensure_platform_capability_schema,
    get_platform_capability,
)
from utils.publish_task_manager import PublishTaskManager


class PlatformCapabilityCacheTests(unittest.TestCase):
    def test_cache_platform_capabilities_from_session_payload_updates_local_snapshot(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="platform-capabilities-cache-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        ensure_platform_capability_schema(db_path=db_path)
        cache_platform_capabilities_from_session_payload(
            {
                "device": {
                    "platformCapabilitiesRevision": "rev-2",
                    "platformCapabilities": [
                        {
                            "platformType": 1,
                            "slug": "xiaohongshu",
                            "label": "小红书",
                            "displayOrder": 40,
                            "visible": True,
                            "loginEnabled": True,
                            "publishEnabled": False,
                            "disabledReason": "等待 OTA 放开发布",
                        }
                    ],
                }
            },
            db_path=db_path,
        )

        capability = get_platform_capability(1, db_path=db_path)

        self.assertIsNotNone(capability)
        self.assertTrue(capability["loginEnabled"])
        self.assertFalse(capability["publishEnabled"])
        self.assertEqual(capability["disabledReason"], "等待 OTA 放开发布")
        self.assertEqual(capability["sourceRevision"], "rev-2")


class PublishTaskPlatformGuardTests(unittest.TestCase):
    def test_publish_task_manager_rejects_disabled_publish_platform(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="platform-capabilities-disabled-platform-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        manager.init_db()

        with self.assertRaisesRegex(ValueError, "小红书发布暂未开放"):
            manager._build_task_specs(
                {
                    "type": 1,
                    "title": "beta blocked platform",
                }
            )

    def test_publish_task_manager_live_account_preflight_marks_login_required_as_needs_verify(self):
        temp_dir = Path(tempfile.mkdtemp(prefix="platform-capabilities-account-preflight-"))
        self.addCleanup(lambda: shutil.rmtree(temp_dir, ignore_errors=True))
        db_path = temp_dir / "database.db"

        manager = PublishTaskManager(db_path=db_path, material_roots={})
        manager.init_db()

        with sqlite3.connect(db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                CREATE TABLE user_info (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    type INTEGER NOT NULL,
                    filePath TEXT NOT NULL,
                    userName TEXT NOT NULL,
                    status INTEGER DEFAULT 1
                )
                """
            )
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (3, "cookies/douyin-demo.json", "抖音测试账号", 1),
            )
            conn.commit()

        with mock.patch(
            "utils.publish_task_manager.check_cookie_detail",
            new=mock.AsyncMock(
                return_value={
                    "ok": False,
                    "state": "login_required",
                    "message": "本地 cookie 已失效，需要重新扫码登录",
                }
            ),
        ):
            result = manager._check_account_status("cookies/douyin-demo.json")

        self.assertFalse(result["ok"])
        self.assertEqual(result["taskStatus"], "needs_verify")
        self.assertIn("重新扫码登录", result["message"])

        with sqlite3.connect(db_path) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                "SELECT status, lastValidationMessage FROM user_info WHERE filePath = ?",
                ("cookies/douyin-demo.json",),
            )
            row = cursor.fetchone()

        self.assertEqual(int(row["status"]), 0)
        self.assertIn("重新扫码登录", row["lastValidationMessage"])
