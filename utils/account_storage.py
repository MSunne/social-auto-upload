import json
import sqlite3
import uuid
from pathlib import Path

from conf import BASE_DIR


USER_INFO_STORAGE_COLUMNS = {
    "storageStateJson": "TEXT",
    "storageStateUpdatedAt": "DATETIME",
}


def get_account_db_path(base_dir=None):
    root = Path(base_dir or BASE_DIR)
    return root / "db" / "database.db"


def get_cookie_dir(base_dir=None):
    root = Path(base_dir or BASE_DIR)
    return root / "cookiesFile"


def ensure_account_storage_schema(db_path=None, conn=None):
    owns_connection = conn is None
    if owns_connection:
        conn = sqlite3.connect(db_path or get_account_db_path())

    try:
        cursor = conn.cursor()
        cursor.execute(
            """
            SELECT name
            FROM sqlite_master
            WHERE type = 'table' AND name = 'user_info'
            """
        )
        if cursor.fetchone() is None:
            return
        cursor.execute("PRAGMA table_info(user_info)")
        existing_columns = {row[1] for row in cursor.fetchall()}
        for column_name, column_type in USER_INFO_STORAGE_COLUMNS.items():
            if column_name in existing_columns:
                continue
            cursor.execute(f"ALTER TABLE user_info ADD COLUMN {column_name} {column_type}")
        if owns_connection:
            conn.commit()
    finally:
        if owns_connection:
            conn.close()


def _normalize_row(row):
    if row is None:
        return None
    if isinstance(row, dict):
        return dict(row)
    try:
        return dict(row)
    except Exception:
        return None


def _normalize_storage_state(storage_state):
    if storage_state is None:
        return None
    if isinstance(storage_state, (dict, list)):
        return storage_state
    if isinstance(storage_state, (bytes, bytearray)):
        storage_state = storage_state.decode("utf-8")
    if isinstance(storage_state, str):
        normalized = storage_state.strip()
        if not normalized:
            return None
        return json.loads(normalized)
    raise TypeError(f"Unsupported storage state type: {type(storage_state)!r}")


def dumps_storage_state(storage_state):
    normalized = _normalize_storage_state(storage_state)
    if normalized is None:
        return None
    return json.dumps(normalized, ensure_ascii=False)


def parse_storage_state(storage_state):
    normalized = _normalize_storage_state(storage_state)
    if normalized is None:
        return None
    if not isinstance(normalized, dict):
        raise ValueError("storage_state 必须是对象")
    return normalized


def is_storage_state_payload(value):
    return isinstance(value, dict) and ("cookies" in value or "origins" in value)


def resolve_account_storage_state(account_ref, db_path=None, base_dir=None, migrate_legacy=True):
    if is_storage_state_payload(account_ref):
        return parse_storage_state(account_ref)
    return load_account_storage_state(account_ref, db_path=db_path, base_dir=base_dir, migrate_legacy=migrate_legacy)


def _resolve_selector(account_ref, base_dir=None):
    base_dir = Path(base_dir or BASE_DIR)
    cookie_dir = get_cookie_dir(base_dir).resolve()

    if isinstance(account_ref, int) and not isinstance(account_ref, bool):
        return {"id": int(account_ref)}

    row = _normalize_row(account_ref)
    if row:
        selector = {}
        if row.get("id") is not None:
            try:
                selector["id"] = int(row["id"])
            except (TypeError, ValueError):
                pass
        if row.get("filePath"):
            selector["filePath"] = str(row["filePath"]).strip()
        if selector:
            return selector

    raw = account_ref
    if isinstance(account_ref, Path):
        raw = str(account_ref)
    raw = str(raw or "").strip()
    if not raw:
        return {}

    candidate = Path(raw).expanduser()
    if candidate.is_absolute():
        resolved = candidate.resolve()
        try:
            relative = resolved.relative_to(cookie_dir).as_posix()
            return {"filePath": relative, "absolutePath": str(resolved)}
        except ValueError:
            return {"absolutePath": str(resolved)}

    normalized = raw.replace("\\", "/")
    return {"filePath": normalized}


def get_account_row(account_ref, db_path=None, base_dir=None):
    selector = _resolve_selector(account_ref, base_dir=base_dir)
    if not selector:
        return None

    ensure_account_storage_schema(db_path=db_path)
    with sqlite3.connect(db_path or get_account_db_path(base_dir)) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        if selector.get("id") is not None:
            cursor.execute("SELECT * FROM user_info WHERE id = ?", (selector["id"],))
        elif selector.get("filePath"):
            cursor.execute("SELECT * FROM user_info WHERE filePath = ?", (selector["filePath"],))
        else:
            return None
        return _normalize_row(cursor.fetchone())


def load_storage_state_from_file(account_ref, base_dir=None):
    selector = _resolve_selector(account_ref, base_dir=base_dir)
    candidate = None
    if selector.get("absolutePath"):
        candidate = Path(selector["absolutePath"])
    elif selector.get("filePath"):
        candidate = get_cookie_dir(base_dir) / selector["filePath"]
    elif selector.get("id") is not None:
        row = get_account_row(account_ref, base_dir=base_dir)
        if row and row.get("filePath"):
            candidate = get_cookie_dir(base_dir) / str(row["filePath"]).strip()
    if not candidate or not candidate.exists() or not candidate.is_file():
        return None
    return parse_storage_state(candidate.read_text(encoding="utf-8"))


def save_storage_state_to_file(account_ref, storage_state, base_dir=None):
    selector = _resolve_selector(account_ref, base_dir=base_dir)
    candidate = None
    if selector.get("absolutePath"):
        candidate = Path(selector["absolutePath"])
    elif selector.get("filePath"):
        candidate = get_cookie_dir(base_dir) / selector["filePath"]
    if not candidate:
        raise ValueError("缺少可写入的账号文件路径")
    candidate.parent.mkdir(parents=True, exist_ok=True)
    candidate.write_text(json.dumps(parse_storage_state(storage_state), ensure_ascii=False, indent=2), encoding="utf-8")
    return candidate


def load_account_storage_state(account_ref, db_path=None, base_dir=None, migrate_legacy=True):
    row = get_account_row(account_ref, db_path=db_path, base_dir=base_dir)
    if row and row.get("storageStateJson"):
        return parse_storage_state(row["storageStateJson"])

    legacy_state = load_storage_state_from_file(account_ref, base_dir=base_dir)
    if legacy_state and row and migrate_legacy:
        update_account_storage_state(row, legacy_state, db_path=db_path, base_dir=base_dir)
    return legacy_state


def account_storage_exists(account_ref, db_path=None, base_dir=None):
    return load_account_storage_state(account_ref, db_path=db_path, base_dir=base_dir, migrate_legacy=False) is not None


def update_account_storage_state(account_ref, storage_state, db_path=None, base_dir=None):
    row = get_account_row(account_ref, db_path=db_path, base_dir=base_dir)
    if not row:
        selector = _resolve_selector(account_ref, base_dir=base_dir)
        if selector.get("absolutePath") or selector.get("filePath"):
            save_storage_state_to_file(selector.get("absolutePath") or selector.get("filePath"), storage_state, base_dir=base_dir)
            return None
        raise ValueError("账号不存在，无法更新 storage_state")

    ensure_account_storage_schema(db_path=db_path)
    serialized = dumps_storage_state(storage_state)
    with sqlite3.connect(db_path or get_account_db_path(base_dir)) as conn:
        cursor = conn.cursor()
        cursor.execute(
            """
            UPDATE user_info
            SET storageStateJson = ?,
                storageStateUpdatedAt = CURRENT_TIMESTAMP
            WHERE id = ?
            """,
            (serialized, int(row["id"])),
        )
        conn.commit()
    return get_account_row(row, db_path=db_path, base_dir=base_dir)


def upsert_login_account(account_type, user_name, file_name=None, status=1, storage_state=None, db_path=None, base_dir=None):
    ensure_account_storage_schema(db_path=db_path)
    serialized = dumps_storage_state(storage_state) if storage_state is not None else None
    fallback_key = str(file_name or f"{uuid.uuid4()}.json").strip() or f"{uuid.uuid4()}.json"

    with sqlite3.connect(db_path or get_account_db_path(base_dir)) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        cursor.execute(
            """
            SELECT id, filePath
            FROM user_info
            WHERE type = ? AND userName = ?
            ORDER BY id ASC
            """,
            (int(account_type), str(user_name).strip()),
        )
        rows = cursor.fetchall()
        if rows:
            primary = rows[0]
            storage_key = str(primary["filePath"] or fallback_key).strip() or fallback_key
            duplicate_ids = [row["id"] for row in rows[1:]]
            if serialized is None:
                cursor.execute(
                    """
                    UPDATE user_info
                    SET type = ?, filePath = ?, userName = ?, status = ?
                    WHERE id = ?
                    """,
                    (int(account_type), storage_key, str(user_name).strip(), int(status), int(primary["id"])),
                )
            else:
                cursor.execute(
                    """
                    UPDATE user_info
                    SET type = ?, filePath = ?, userName = ?, status = ?,
                        storageStateJson = ?, storageStateUpdatedAt = CURRENT_TIMESTAMP
                    WHERE id = ?
                    """,
                    (
                        int(account_type),
                        storage_key,
                        str(user_name).strip(),
                        int(status),
                        serialized,
                        int(primary["id"]),
                    ),
                )
            if duplicate_ids:
                cursor.executemany("DELETE FROM user_info WHERE id = ?", [(int(item),) for item in duplicate_ids])
        else:
            storage_key = fallback_key
            cursor.execute(
                """
                INSERT INTO user_info (type, filePath, userName, status, storageStateJson, storageStateUpdatedAt)
                VALUES (?, ?, ?, ?, ?, CASE WHEN ? IS NULL THEN NULL ELSE CURRENT_TIMESTAMP END)
                """,
                (
                    int(account_type),
                    storage_key,
                    str(user_name).strip(),
                    int(status),
                    serialized,
                    serialized,
                ),
            )
        conn.commit()
    return storage_key


def clear_account_storage_state(account_ref, db_path=None, base_dir=None):
    row = get_account_row(account_ref, db_path=db_path, base_dir=base_dir)
    if row:
        ensure_account_storage_schema(db_path=db_path)
        with sqlite3.connect(db_path or get_account_db_path(base_dir)) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE user_info
                SET storageStateJson = NULL,
                    storageStateUpdatedAt = NULL
                WHERE id = ?
                """,
                (int(row["id"]),),
            )
            conn.commit()

    legacy_path = _resolve_selector(account_ref, base_dir=base_dir)
    if legacy_path.get("absolutePath"):
        candidate = Path(legacy_path["absolutePath"])
    elif legacy_path.get("filePath"):
        candidate = get_cookie_dir(base_dir) / legacy_path["filePath"]
    else:
        candidate = None
    if candidate and candidate.exists():
        try:
            candidate.unlink()
        except OSError:
            pass


def export_account_storage_state(account_ref, db_path=None, base_dir=None):
    storage_state = load_account_storage_state(account_ref, db_path=db_path, base_dir=base_dir)
    if storage_state is None:
        raise FileNotFoundError("账号登录态不存在")
    return json.dumps(storage_state, ensure_ascii=False, indent=2).encode("utf-8")


def import_account_storage_state(account_ref, payload, db_path=None, base_dir=None):
    storage_state = parse_storage_state(payload)
    update_account_storage_state(account_ref, storage_state, db_path=db_path, base_dir=base_dir)
    return storage_state


def has_persisted_storage_state(row):
    normalized = _normalize_row(row) or {}
    return bool(normalized.get("storageStateJson"))
