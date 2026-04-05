import sqlite3
from pathlib import Path

from conf import BASE_DIR


DEFAULT_PLATFORM_CAPABILITIES = (
    {
        "platformType": 3,
        "slug": "douyin",
        "label": "抖音",
        "aliases": ("douyin", "抖音"),
        "displayOrder": 10,
        "visible": True,
        "loginEnabled": True,
        "publishEnabled": True,
        "disabledReason": None,
    },
    {
        "platformType": 4,
        "slug": "kuaishou",
        "label": "快手",
        "aliases": ("kuaishou", "快手"),
        "displayOrder": 20,
        "visible": True,
        "loginEnabled": True,
        "publishEnabled": True,
        "disabledReason": None,
    },
    {
        "platformType": 2,
        "slug": "wechat_channel",
        "label": "视频号",
        "aliases": ("wechat_channel", "wechat", "shipinhao", "视频号"),
        "displayOrder": 30,
        "visible": True,
        "loginEnabled": True,
        "publishEnabled": True,
        "disabledReason": None,
    },
    {
        "platformType": 1,
        "slug": "xiaohongshu",
        "label": "小红书",
        "aliases": ("xiaohongshu", "小红书"),
        "displayOrder": 40,
        "visible": True,
        "loginEnabled": False,
        "publishEnabled": False,
        "disabledReason": "本期未开放",
    },
)

PLATFORM_SPEC_BY_TYPE = {
    int(item["platformType"]): dict(item)
    for item in DEFAULT_PLATFORM_CAPABILITIES
}
PLATFORM_NAME_BY_TYPE = {
    int(item["platformType"]): str(item["label"]).strip()
    for item in DEFAULT_PLATFORM_CAPABILITIES
}
PLATFORM_LABELS = dict(PLATFORM_NAME_BY_TYPE)
PLATFORM_LABELS_STR = {
    str(platform_type): label
    for platform_type, label in PLATFORM_NAME_BY_TYPE.items()
}
PLATFORM_SLUG_BY_TYPE = {
    int(item["platformType"]): str(item["slug"]).strip()
    for item in DEFAULT_PLATFORM_CAPABILITIES
}
PLATFORM_TYPE_BY_NAME = {
    str(item["label"]).strip(): int(item["platformType"])
    for item in DEFAULT_PLATFORM_CAPABILITIES
}
PLATFORM_ALIAS_MAP = {}
for item in DEFAULT_PLATFORM_CAPABILITIES:
    aliases = {
        str(alias or "").strip().lower()
        for alias in item.get("aliases") or ()
        if str(alias or "").strip()
    }
    aliases.add(str(item["slug"]).strip().lower())
    aliases.add(str(item["label"]).strip().lower())
    for alias in aliases:
        PLATFORM_ALIAS_MAP[alias] = {
            "slug": str(item["slug"]).strip(),
            "type": int(item["platformType"]),
            "label": str(item["label"]).strip(),
        }


def get_platform_db_path(db_path=None):
    return Path(db_path or (Path(BASE_DIR) / "db" / "database.db"))


def _normalize_bool(value, default=False):
    if value is None:
        return bool(default)
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    return str(value).strip().lower() in {"1", "true", "yes", "on"}


def resolve_platform_spec(platform_type=None, slug=None, label=None, value=None):
    if platform_type not in (None, ""):
        try:
            normalized_type = int(platform_type)
        except (TypeError, ValueError):
            normalized_type = None
        if normalized_type in PLATFORM_SPEC_BY_TYPE:
            return dict(PLATFORM_SPEC_BY_TYPE[normalized_type])

    for candidate in (slug, label, value):
        normalized = str(candidate or "").strip().lower()
        if not normalized:
            continue
        resolved = PLATFORM_ALIAS_MAP.get(normalized)
        if not resolved:
            continue
        return dict(PLATFORM_SPEC_BY_TYPE[int(resolved["type"])])
    return None


def build_platform_capability_payload(spec, *, source_revision=None, overrides=None):
    normalized = dict(spec or {})
    result = {
        "platformType": int(normalized["platformType"]),
        "slug": str(normalized["slug"]).strip(),
        "label": str(normalized["label"]).strip(),
        "displayOrder": int(normalized.get("displayOrder") or 0),
        "visible": _normalize_bool(normalized.get("visible"), True),
        "loginEnabled": _normalize_bool(normalized.get("loginEnabled"), True),
        "publishEnabled": _normalize_bool(normalized.get("publishEnabled"), True),
        "disabledReason": str(normalized.get("disabledReason") or "").strip() or None,
        "sourceRevision": str(
            source_revision
            if source_revision not in (None, "")
            else normalized.get("sourceRevision") or ""
        ).strip() or None,
    }
    if overrides:
        result.update(overrides)
    return result


def normalize_platform_capability(item, *, source_revision=None):
    if not isinstance(item, dict):
        raise ValueError("platform capability payload must be a dict")

    spec = resolve_platform_spec(
        platform_type=item.get("platformType") or item.get("type"),
        slug=item.get("slug") or item.get("platform"),
        label=item.get("label") or item.get("platformName") or item.get("name"),
    )
    if not spec:
        raise ValueError(f"unsupported platform capability: {item}")

    overrides = {
        "visible": _normalize_bool(item.get("visible"), spec.get("visible", True)),
        "loginEnabled": _normalize_bool(item.get("loginEnabled"), spec.get("loginEnabled", True)),
        "publishEnabled": _normalize_bool(item.get("publishEnabled"), spec.get("publishEnabled", True)),
        "disabledReason": str(item.get("disabledReason") or spec.get("disabledReason") or "").strip() or None,
        "displayOrder": int(item.get("displayOrder") or spec.get("displayOrder") or 0),
    }
    return build_platform_capability_payload(spec, source_revision=source_revision, overrides=overrides)


def ensure_platform_capability_schema(db_path=None, conn=None):
    owns_connection = conn is None
    if owns_connection:
        conn = sqlite3.connect(get_platform_db_path(db_path))

    try:
        cursor = conn.cursor()
        cursor.execute(
            """
            CREATE TABLE IF NOT EXISTS platform_capabilities (
                platform_type INTEGER PRIMARY KEY,
                slug TEXT NOT NULL UNIQUE,
                label TEXT NOT NULL,
                display_order INTEGER NOT NULL DEFAULT 0,
                visible INTEGER NOT NULL DEFAULT 1,
                login_enabled INTEGER NOT NULL DEFAULT 1,
                publish_enabled INTEGER NOT NULL DEFAULT 1,
                disabled_reason TEXT,
                source_revision TEXT,
                updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
            )
            """
        )

        for column_sql in (
            "ALTER TABLE platform_capabilities ADD COLUMN display_order INTEGER NOT NULL DEFAULT 0",
            "ALTER TABLE platform_capabilities ADD COLUMN visible INTEGER NOT NULL DEFAULT 1",
            "ALTER TABLE platform_capabilities ADD COLUMN login_enabled INTEGER NOT NULL DEFAULT 1",
            "ALTER TABLE platform_capabilities ADD COLUMN publish_enabled INTEGER NOT NULL DEFAULT 1",
            "ALTER TABLE platform_capabilities ADD COLUMN disabled_reason TEXT",
            "ALTER TABLE platform_capabilities ADD COLUMN source_revision TEXT",
            "ALTER TABLE platform_capabilities ADD COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP",
        ):
            try:
                cursor.execute(column_sql)
            except sqlite3.OperationalError:
                pass

        for item in DEFAULT_PLATFORM_CAPABILITIES:
            payload = build_platform_capability_payload(item)
            cursor.execute(
                """
                INSERT OR IGNORE INTO platform_capabilities (
                    platform_type, slug, label, display_order, visible, login_enabled,
                    publish_enabled, disabled_reason, source_revision
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
                """,
                (
                    payload["platformType"],
                    payload["slug"],
                    payload["label"],
                    payload["displayOrder"],
                    1 if payload["visible"] else 0,
                    1 if payload["loginEnabled"] else 0,
                    1 if payload["publishEnabled"] else 0,
                    payload["disabledReason"],
                    payload["sourceRevision"],
                ),
            )
        if owns_connection:
            conn.commit()
    finally:
        if owns_connection:
            conn.close()


def upsert_platform_capabilities(items, *, source_revision=None, db_path=None):
    ensure_platform_capability_schema(db_path=db_path)
    normalized_items = [
        normalize_platform_capability(item, source_revision=source_revision)
        for item in (items or [])
        if isinstance(item, dict)
    ]
    if not normalized_items:
        return list_platform_capabilities(db_path=db_path, visible_only=False)

    with sqlite3.connect(get_platform_db_path(db_path)) as conn:
        cursor = conn.cursor()
        for item in normalized_items:
            cursor.execute(
                """
                INSERT INTO platform_capabilities (
                    platform_type, slug, label, display_order, visible, login_enabled,
                    publish_enabled, disabled_reason, source_revision, updated_at
                )
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
                ON CONFLICT(platform_type) DO UPDATE SET
                    slug = excluded.slug,
                    label = excluded.label,
                    display_order = excluded.display_order,
                    visible = excluded.visible,
                    login_enabled = excluded.login_enabled,
                    publish_enabled = excluded.publish_enabled,
                    disabled_reason = excluded.disabled_reason,
                    source_revision = excluded.source_revision,
                    updated_at = CURRENT_TIMESTAMP
                """,
                (
                    item["platformType"],
                    item["slug"],
                    item["label"],
                    item["displayOrder"],
                    1 if item["visible"] else 0,
                    1 if item["loginEnabled"] else 0,
                    1 if item["publishEnabled"] else 0,
                    item["disabledReason"],
                    item["sourceRevision"],
                ),
            )
        conn.commit()

    return list_platform_capabilities(db_path=db_path, visible_only=False)


def list_platform_capabilities(db_path=None, *, visible_only=False):
    ensure_platform_capability_schema(db_path=db_path)
    with sqlite3.connect(get_platform_db_path(db_path)) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        query = """
            SELECT platform_type, slug, label, display_order, visible, login_enabled,
                   publish_enabled, disabled_reason, source_revision, updated_at
            FROM platform_capabilities
        """
        params = []
        if visible_only:
            query += " WHERE visible = 1"
        query += " ORDER BY display_order ASC, platform_type ASC"
        cursor.execute(query, params)
        rows = cursor.fetchall()

    return [
        {
            "platformType": int(row["platform_type"]),
            "slug": str(row["slug"]).strip(),
            "label": str(row["label"]).strip(),
            "displayOrder": int(row["display_order"] or 0),
            "visible": bool(row["visible"]),
            "loginEnabled": bool(row["login_enabled"]),
            "publishEnabled": bool(row["publish_enabled"]),
            "disabledReason": str(row["disabled_reason"] or "").strip() or None,
            "sourceRevision": str(row["source_revision"] or "").strip() or None,
            "updatedAt": row["updated_at"],
        }
        for row in rows
    ]


def get_platform_capability(platform_type, *, db_path=None):
    try:
        normalized_type = int(platform_type)
    except (TypeError, ValueError):
        return None

    for item in list_platform_capabilities(db_path=db_path, visible_only=False):
        if int(item["platformType"]) == normalized_type:
            return item
    return None


def get_visible_platform_capabilities(db_path=None):
    return list_platform_capabilities(db_path=db_path, visible_only=True)


def cache_platform_capabilities_from_session_payload(payload, *, db_path=None):
    if not isinstance(payload, dict):
        return []

    device = payload.get("device") if isinstance(payload.get("device"), dict) else {}
    items = device.get("platformCapabilities") if isinstance(device.get("platformCapabilities"), list) else []
    source_revision = str(device.get("platformCapabilitiesRevision") or "").strip() or None
    if not items:
        return list_platform_capabilities(db_path=db_path, visible_only=False)
    return upsert_platform_capabilities(items, source_revision=source_revision, db_path=db_path)


def format_platform_unavailable_message(capability, operation):
    if not capability:
        return "不支持的平台类型"
    label = str(capability.get("label") or "当前平台").strip()
    reason = str(capability.get("disabledReason") or "").strip() or "本期未开放"
    action = "登录" if operation == "login" else "发布"
    return f"{label}{action}暂未开放：{reason}"


def ensure_platform_operation_enabled(platform_type, operation, *, db_path=None):
    capability = get_platform_capability(platform_type, db_path=db_path)
    if not capability:
        raise ValueError("不支持的平台类型")

    if operation == "login" and not capability.get("loginEnabled"):
        raise ValueError(format_platform_unavailable_message(capability, operation))
    if operation == "publish" and not capability.get("publishEnabled"):
        raise ValueError(format_platform_unavailable_message(capability, operation))
    return capability
