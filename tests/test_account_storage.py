import json
import sqlite3
import tempfile
import unittest
from pathlib import Path

from utils.account_storage import (
    account_storage_exists,
    clear_account_storage_state,
    ensure_account_storage_schema,
    export_account_storage_state,
    get_account_row,
    import_account_storage_state,
    load_account_storage_state,
    upsert_login_account,
)


SAMPLE_STORAGE_STATE = {
    "cookies": [
        {
            "name": "sessionid",
            "value": "abc123",
            "domain": ".example.com",
            "path": "/",
            "expires": -1,
            "httpOnly": True,
            "secure": True,
            "sameSite": "Lax",
        }
    ],
    "origins": [],
}


class AccountStorageTestCase(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.base_dir = Path(self.temp_dir.name)
        (self.base_dir / "db").mkdir(parents=True, exist_ok=True)
        (self.base_dir / "cookiesFile").mkdir(parents=True, exist_ok=True)
        self.db_path = self.base_dir / "db" / "database.db"

        with sqlite3.connect(self.db_path) as conn:
            conn.execute(
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
            conn.commit()

    def tearDown(self):
        self.temp_dir.cleanup()

    def _get_columns(self):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("PRAGMA table_info(user_info)")
            return [row[1] for row in cursor.fetchall()]

    def test_schema_migration_and_round_trip_storage(self):
        ensure_account_storage_schema(db_path=self.db_path)
        columns = self._get_columns()
        self.assertIn("storageStateJson", columns)
        self.assertIn("storageStateUpdatedAt", columns)

        storage_key = upsert_login_account(
            4,
            "测试快手_乔总",
            file_name="kuaishou.json",
            status=1,
            storage_state=SAMPLE_STORAGE_STATE,
            db_path=self.db_path,
            base_dir=self.base_dir,
        )
        self.assertEqual(storage_key, "kuaishou.json")

        row = get_account_row("kuaishou.json", db_path=self.db_path, base_dir=self.base_dir)
        self.assertEqual(row["userName"], "测试快手_乔总")
        self.assertTrue(account_storage_exists(row, db_path=self.db_path, base_dir=self.base_dir))

        exported = export_account_storage_state(row["id"], db_path=self.db_path, base_dir=self.base_dir)
        self.assertEqual(json.loads(exported.decode("utf-8")), SAMPLE_STORAGE_STATE)

        clear_account_storage_state(row, db_path=self.db_path, base_dir=self.base_dir)
        cleared_row = get_account_row(row["id"], db_path=self.db_path, base_dir=self.base_dir)
        self.assertIsNone(cleared_row["storageStateJson"])
        self.assertFalse(account_storage_exists(row["id"], db_path=self.db_path, base_dir=self.base_dir))

        import_account_storage_state(row["id"], exported, db_path=self.db_path, base_dir=self.base_dir)
        self.assertEqual(
            load_account_storage_state(row["id"], db_path=self.db_path, base_dir=self.base_dir),
            SAMPLE_STORAGE_STATE,
        )

    def test_legacy_file_can_be_loaded_and_migrated_by_row_selector(self):
        ensure_account_storage_schema(db_path=self.db_path)

        with sqlite3.connect(self.db_path) as conn:
            conn.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                """,
                (3, "legacy-douyin.json", "姜姜总裁抖音", 1),
            )
            conn.commit()

        legacy_file = self.base_dir / "cookiesFile" / "legacy-douyin.json"
        legacy_file.write_text(json.dumps(SAMPLE_STORAGE_STATE, ensure_ascii=False), encoding="utf-8")

        row = get_account_row(1, db_path=self.db_path, base_dir=self.base_dir)
        selector = {"id": row["id"], "filePath": row["filePath"]}
        self.assertTrue(account_storage_exists(selector, db_path=self.db_path, base_dir=self.base_dir))

        loaded = load_account_storage_state(selector, db_path=self.db_path, base_dir=self.base_dir)
        self.assertEqual(loaded, SAMPLE_STORAGE_STATE)

        migrated_row = get_account_row(row["id"], db_path=self.db_path, base_dir=self.base_dir)
        self.assertIsNotNone(migrated_row["storageStateJson"])
        self.assertEqual(json.loads(migrated_row["storageStateJson"]), SAMPLE_STORAGE_STATE)


if __name__ == "__main__":
    unittest.main()
