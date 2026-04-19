import asyncio
import ipaddress
import json
import os
import re
import secrets
import sqlite3
import subprocess
import threading
import time
import uuid
import socket
from datetime import datetime, timedelta, timezone
from urllib import error as urllib_error
from urllib.parse import urlencode
from urllib import request as urllib_request
from pathlib import Path
from queue import Empty, Queue
import conf as app_conf
from myUtils.auth import check_cookie_detail
from flask import Flask, request, jsonify, Response, render_template, send_from_directory
from conf import BASE_DIR
from myUtils.login import (
    get_tencent_cookie,
    douyin_cookie_gen,
    get_ks_cookie,
    get_backend_entry_config,
    open_platform_backend_session,
    xiaohongshu_cookie_gen,
    push_login_failed_status,
)
from utils.account_storage import (
    account_storage_exists,
    clear_account_storage_state,
    ensure_account_storage_schema,
    export_account_storage_state,
    get_account_row,
    get_cookie_dir,
    has_persisted_storage_state,
    import_account_storage_state,
    update_account_runtime_status,
)
from utils.cloud_agent import CloudAgent
from utils.cloud_sync import CloudSyncClient
from utils.device_identity import load_device_identity
from utils.device_meta import get_device_code
from utils.materials import (
    build_material_roots,
    list_material_directory,
    list_material_roots,
    read_material_file,
    resolve_material_reference,
)
from utils.omnidrive_agent import OmniDriveBridge
from utils.omnidrive_ai_task_manager import OmniDriveAITaskManager
from utils.platform_capabilities import (
    PLATFORM_LABELS,
    PLATFORM_LABELS_STR,
    cache_platform_capabilities_from_session_payload,
    ensure_platform_capability_schema,
    ensure_platform_operation_enabled,
    format_platform_unavailable_message,
    get_platform_capability,
    get_visible_platform_capabilities,
)
from utils.publish_task_manager import PublishTaskManager
from utils.runtime_health import build_runtime_health, log_runtime_health
from utils.log import (
    agent_logger,
    ai_logger,
    app_logger,
    login_logger,
    log_throttled,
    request_logger,
    task_logger,
)

active_queues = {}
active_backend_sessions = {}
backend_session_lock = threading.Lock()
app = Flask(__name__)
ensure_account_storage_schema()
ensure_platform_capability_schema()
NOISY_REQUEST_INTERVALS = {
    '/cloudAgentStatus': 20,
    '/omnidriveAgentStatus': 20,
    '/api/skill/status': 15,
    '/favicon.ico': 300,
    '/vite.svg': 300,
}
NOISY_REQUEST_PREFIX_INTERVALS = {
    '/assets/': 300,
}
REQUEST_LOG_BODY_LIMIT = 500
OMNIDRIVE_OPENAI_PROXY_BASE_PATH = "/openai/v1"
OMNIDRIVE_OPENAI_PROXY_FINAL_JOB_STATUSES = {"success", "completed", "failed", "cancelled", "needs_verify"}
OMNIDRIVE_OPENAI_SUPPORTED_TOOL_NAMES = {"omnidrive_image", "omnidrive_video", "omnidrive_mix_video"}
OPENCLAW_OMNIDRIVE_CONFIG_PATHS = (
    Path.home() / ".openclaw" / "openclaw.json",
    Path.home() / ".openclaw" / "agents" / "main" / "agent" / "models.json",
)
OPENCLAW_OMNIDRIVE_MODEL_NAME_OVERRIDES = {
    "gemini-3.1-pro-preview": "Gemini 3.1 Pro Preview",
    "gpt-5.4": "GPT-5.4",
    "qwen3.5-plus": "Qwen3.5 Plus",
}
OPENCLAW_OMNIDRIVE_MULTIMODAL_MODELS = {
    "gemini-3.1-pro-preview",
    "gpt-5.4",
    "qwen3.5-plus",
}
OPENCLAW_OMNIDRIVE_AVAILABLE_SKILLS = [
    "omnidrive_auth",
    "omnidrive_models",
    "omnidrive_chat",
    "omnidrive_image",
    "omnidrive_video",
    "omnidrive_mix_video",
    "omnidrive_jobs",
    "omnidrive_job_detail",
    "omnibull_status",
    "omnibull_accounts",
    "omnibull_materials",
    "omnibull_publish",
]
OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS = max(
    60,
    int(getattr(app_conf, "OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS", 300)),
)
OPENCLAW_OMNIDRIVE_MODEL_SYNC_INTERVAL_SECONDS = max(
    OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS,
    int(getattr(app_conf, "OPENCLAW_OMNIDRIVE_MODEL_SYNC_INTERVAL_SECONDS", 300)),
)
OPENCLAW_OMNIDRIVE_TOKEN_REFRESH_MARGIN_SECONDS = max(
    30,
    int(getattr(app_conf, "OPENCLAW_OMNIDRIVE_TOKEN_REFRESH_MARGIN_SECONDS", 300)),
)
OMNIDRIVE_OPENAI_LOCAL_TOOL_GUARD_TEXT = str(
    getattr(
        app_conf,
        "OMNIDRIVE_OPENAI_LOCAL_TOOL_GUARD_TEXT",
        os.getenv(
            "OMNIDRIVE_OPENAI_LOCAL_TOOL_GUARD_TEXT",
            (
                "当前这条 OmniDrive 默认聊天接入只支持普通对话，以及 OmniDrive 图片和视频生成。"
                "\n它不支持 OpenClaw 的本地工具，例如 read、write、edit、exec、process 或 web_search。"
                "\n所以我不能真实读取本地文件、重启服务、修改配置或列出本地技能，也不会假装这些动作已经执行。"
                "\n如果你要继续用 OmniDrive，请直接给我图片或视频生成需求；如果你要做本地运维或代码操作，请切回支持 OpenClaw 本地工具的系统模型。"
            ),
        ),
    )
    or ""
).strip()
OMNIDRIVE_OPENAI_LOCAL_TOOL_SYSTEM_GUARD = str(
    getattr(
        app_conf,
        "OMNIDRIVE_OPENAI_LOCAL_TOOL_SYSTEM_GUARD",
        os.getenv(
            "OMNIDRIVE_OPENAI_LOCAL_TOOL_SYSTEM_GUARD",
            (
                "This OmniDrive route cannot execute OpenClaw local tools such as read, write, edit, exec, "
                "process, or web_search. Never claim that you ran commands, restarted services, read local "
                "files, listed local skills, or modified configuration. If asked to do that, clearly say this "
                "route currently supports plain chat plus OmniDrive image/video generation only."
            ),
        ),
    )
    or ""
).strip()
SOURCE_CATEGORY_OPENCLAW_DIRECT = "openclaw_direct"
SOURCE_CATEGORY_OPENCLAW_VIA_HERMES = "openclaw_via_hermes"
SOURCE_CATEGORY_HERMES_DIRECT = "hermes_direct"
SOURCE_CATEGORY_HERMES_SCHEDULED = "hermes_scheduled"
HERMES_SHARED_RUNTIME_VERSION = "omnibull-hermes-shared-runtime-v1"


def parse_bool(value):
    """Normalize environment/config values into a predictable boolean flag."""
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    if isinstance(value, (int, float)):
        return value != 0
    return str(value).strip().lower() in {'1', 'true', 'yes', 'on'}


OPENCLAW_OMNIDRIVE_RELOAD_GATEWAY_ON_MODEL_SYNC = parse_bool(
    getattr(app_conf, "OPENCLAW_OMNIDRIVE_RELOAD_GATEWAY_ON_MODEL_SYNC", True)
)


def parse_csv(value, default=None):
    """Split comma-separated config values while preserving a fallback default list."""
    if value is None:
        return list(default or [])
    if isinstance(value, (list, tuple, set)):
        return [str(item).strip() for item in value if str(item).strip()]
    parts = [item.strip() for item in str(value).split(',')]
    cleaned = [item for item in parts if item]
    return cleaned or list(default or [])


def _redact_for_log(value):
    """Remove sensitive fields from structured payloads before they are written to logs."""
    sensitive_keys = {
        'password',
        'token',
        'agentKey',
        'authorization',
        'accessToken',
        'writerToken',
        'viewerToken',
        'qrData',
    }
    if isinstance(value, dict):
        sanitized = {}
        for key, item in value.items():
            if str(key).strip() in sensitive_keys:
                sanitized[key] = '[REDACTED]'
            else:
                sanitized[key] = _redact_for_log(item)
        return sanitized
    if isinstance(value, list):
        return [_redact_for_log(item) for item in value]
    return value


def _compact_log_value(value, limit=REQUEST_LOG_BODY_LIMIT):
    """Serialize request/response payloads into a bounded log-friendly string."""
    if value in (None, '', [], {}):
        return None
    try:
        if isinstance(value, (dict, list, tuple)):
            text = json.dumps(_redact_for_log(value), ensure_ascii=False, default=str)
        else:
            text = str(value)
    except Exception:
        text = repr(value)
    text = text.strip()
    if len(text) > limit:
        return text[:limit] + '...(truncated)'
    return text


def _request_log_interval(path):
    """Return the sampling interval used to suppress noisy endpoints in request logs."""
    if path in NOISY_REQUEST_INTERVALS:
        return NOISY_REQUEST_INTERVALS[path]
    for prefix, interval in NOISY_REQUEST_PREFIX_INTERVALS.items():
        if path.startswith(prefix):
            return interval
    return 0


def _request_payload_summary():
    """Build a compact request-body summary for logging non-idempotent API calls."""
    if request.method in {'GET', 'HEAD', 'OPTIONS'}:
        return None

    payload = get_request_payload()
    files = list(request.files.keys()) if request.files else []
    if files:
        payload = dict(payload)
        payload['_files'] = files
    return _compact_log_value(payload)


CLOUD_AGENT_ENABLED = parse_bool(getattr(app_conf, 'CLOUD_AGENT_ENABLED', False))
CLOUD_DEMO_URL = str(getattr(app_conf, 'CLOUD_DEMO_URL', '')).strip()
CLOUD_DEVICE_NAME = str(getattr(app_conf, 'CLOUD_DEVICE_NAME', '')).strip() or None
CLOUD_AGENT_KEY = str(getattr(app_conf, 'CLOUD_AGENT_KEY', '')).strip()
CLOUD_AGENT_POLL_INTERVAL = int(getattr(app_conf, 'CLOUD_AGENT_POLL_INTERVAL', 5))
CLOUD_AGENT_HEARTBEAT_INTERVAL = int(getattr(app_conf, 'CLOUD_AGENT_HEARTBEAT_INTERVAL', 30))
OMNIDRIVE_AGENT_ENABLED = parse_bool(getattr(app_conf, 'OMNIDRIVE_AGENT_ENABLED', False))
OMNIDRIVE_BASE_URL = str(getattr(app_conf, 'OMNIDRIVE_BASE_URL', '')).strip()
OMNIDRIVE_AGENT_KEY = str(getattr(app_conf, 'OMNIDRIVE_AGENT_KEY', '')).strip()
OMNIDRIVE_AGENT_POLL_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_AGENT_POLL_INTERVAL', 5))
OMNIDRIVE_AGENT_AI_POLL_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_AGENT_AI_POLL_INTERVAL', 15))
OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL', 30))
OMNIDRIVE_ACCOUNT_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_ACCOUNT_SYNC_INTERVAL', 60))
OMNIDRIVE_MATERIAL_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_MATERIAL_SYNC_INTERVAL', 1800))
OMNIDRIVE_SKILL_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_SKILL_SYNC_INTERVAL', 120))
OMNIDRIVE_PUBLISH_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_PUBLISH_SYNC_INTERVAL', 5))
OMNIDRIVE_MATERIAL_SYNC_MAX_FILES = int(getattr(app_conf, 'OMNIDRIVE_MATERIAL_SYNC_MAX_FILES', 1000))
DEVICE_IDENTITY = load_device_identity(app_conf, base_dir=BASE_DIR)
OMNIBULL_PUBLISH_WORKERS = int(getattr(app_conf, 'OMNIBULL_PUBLISH_WORKERS', 1))
OMNIBULL_PUBLISH_DISPATCH_INTERVAL_SECONDS = max(
    0,
    int(getattr(app_conf, 'OMNIBULL_PUBLISH_DISPATCH_INTERVAL_SECONDS', 5)),
)
OMNIBULL_TASK_RETENTION_DAYS = int(getattr(app_conf, 'OMNIBULL_TASK_RETENTION_DAYS', 7))
OMNIBULL_API_KEY = DEVICE_IDENTITY.get("localApiKey") or str(getattr(app_conf, 'OMNIBULL_API_KEY', '')).strip()
SAU_BACKEND_PORT = int(os.getenv("SAU_BACKEND_PORT", "5409"))
OMNIBULL_MATERIAL_ROOTS = build_material_roots(
    BASE_DIR,
    getattr(app_conf, 'OMNIBULL_MATERIAL_ROOTS', None),
)
OMNIBULL_CORS_ALLOWED_ORIGINS = parse_csv(
    getattr(app_conf, 'OMNIBULL_CORS_ALLOWED_ORIGINS', '*'),
    default=['*'],
)
OMNIBULL_CORS_ALLOWED_METHODS = parse_csv(
    getattr(app_conf, 'OMNIBULL_CORS_ALLOWED_METHODS', 'GET,POST,PUT,PATCH,DELETE,OPTIONS'),
    default=['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'],
)
OMNIBULL_CORS_ALLOWED_HEADERS = parse_csv(
    getattr(
        app_conf,
        'OMNIBULL_CORS_ALLOWED_HEADERS',
        'Authorization,Content-Type,X-Requested-With,X-Omnibull-Key',
    ),
    default=['Authorization', 'Content-Type', 'X-Requested-With', 'X-Omnibull-Key'],
)
OMNIBULL_CORS_EXPOSE_HEADERS = parse_csv(
    getattr(
        app_conf,
        'OMNIBULL_CORS_EXPOSE_HEADERS',
        'Content-Disposition,X-Accel-Buffering',
    ),
    default=['Content-Disposition', 'X-Accel-Buffering'],
)
OMNIBULL_CORS_ALLOW_CREDENTIALS = parse_bool(
    getattr(app_conf, 'OMNIBULL_CORS_ALLOW_CREDENTIALS', False)
)
OMNIBULL_CORS_MAX_AGE = int(getattr(app_conf, 'OMNIBULL_CORS_MAX_AGE', 86400))
OMNIBULL_GENERATED_ROOT_NAME = str(getattr(app_conf, 'OMNIBULL_GENERATED_ROOT_NAME', 'omnidriveGenerated')).strip() or 'omnidriveGenerated'
OMNIBULL_GENERATED_ROOT_PATH = Path(BASE_DIR / "omnidriveSync" / "generated").resolve()
OMNIBULL_GENERATED_ROOT_PATH.mkdir(parents=True, exist_ok=True)
OMNIBULL_MATERIAL_ROOTS.setdefault(OMNIBULL_GENERATED_ROOT_NAME, OMNIBULL_GENERATED_ROOT_PATH)
HERMES_PROFILE_NAME = str(getattr(app_conf, 'HERMES_PROFILE_NAME', 'omnibull')).strip() or 'omnibull'
HERMES_API_SERVER_BASE_URL = (
    str(getattr(app_conf, 'HERMES_API_SERVER_BASE_URL', '')).strip().rstrip("/")
    or f"http://{str(getattr(app_conf, 'HERMES_API_SERVER_HOST', '127.0.0.1')).strip() or '127.0.0.1'}:{int(getattr(app_conf, 'HERMES_API_SERVER_PORT', 8642))}"
)
HERMES_API_SERVER_KEY = str(getattr(app_conf, 'HERMES_API_SERVER_KEY', '')).strip()
HERMES_API_SERVER_TIMEOUT = max(5, int(getattr(app_conf, 'HERMES_API_SERVER_TIMEOUT', 60)))
HERMES_SHARED_RUNTIME_CONFIG_PATH = Path(
    getattr(
        app_conf,
        'HERMES_SHARED_RUNTIME_CONFIG_PATH',
        BASE_DIR / "runtime" / "agent-runtime" / "hermes-openclaw-runtime.json",
    )
).expanduser()
OPENCLAW_GATEWAY_DAILY_RELOAD_ENABLED = parse_bool(
    getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_ENABLED', True)
)
OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR = min(
    23,
    max(0, int(getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR', 0))),
)
OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE = min(
    59,
    max(0, int(getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE', 0))),
)
OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS = max(
    60,
    int(getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS', 300)),
)
OPENCLAW_GATEWAY_DAILY_RELOAD_TIMEOUT_SECONDS = max(
    30,
    int(getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_TIMEOUT_SECONDS', 180)),
)
OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND = (
    str(getattr(app_conf, 'OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND', 'openclaw gateway restart')).strip()
    or 'openclaw gateway restart'
)
HERMES_LOCAL_BRIDGE_CLIENT_PATH = Path(BASE_DIR / "scripts" / "hermes_local_bridge.py")
OMNIBULL_RUNTIME_HEALTH = build_runtime_health(
    base_dir=BASE_DIR,
    material_roots=OMNIBULL_MATERIAL_ROOTS,
    device_identity_path=DEVICE_IDENTITY.get("path"),
    generated_root_path=OMNIBULL_GENERATED_ROOT_PATH,
    headless=parse_bool(getattr(app_conf, 'LOCAL_CHROME_HEADLESS', False)),
)
RESOLVED_DEVICE_NAME = (
    DEVICE_IDENTITY.get("deviceName")
    or str(getattr(app_conf, 'OMNIBULL_DEVICE_NAME', '')).strip()
    or str(getattr(app_conf, 'OMNIDRIVE_DEVICE_NAME', '')).strip()
    or socket.gethostname()
)
DEVICE_CODE = (
    DEVICE_IDENTITY.get("deviceCode")
    or str(getattr(app_conf, 'OMNIBULL_DEVICE_CODE', '')).strip()
    or str(getattr(app_conf, 'OMNIDRIVE_DEVICE_CODE', '')).strip()
    or get_device_code()
)
OMNIDRIVE_AGENT_KEY = DEVICE_IDENTITY.get("agentKey") or OMNIDRIVE_AGENT_KEY
cloud_agent = None
cloud_agent_lock = threading.Lock()
omnidrive_agent = None
omnidrive_agent_lock = threading.Lock()
omnidrive_ai_task_manager = OmniDriveAITaskManager(Path(BASE_DIR / "db" / "database.db"))
publish_task_manager = PublishTaskManager(
    Path(BASE_DIR / "db" / "database.db"),
    worker_count=OMNIBULL_PUBLISH_WORKERS,
    retention_days=OMNIBULL_TASK_RETENTION_DAYS,
    sync_client=CloudSyncClient(CLOUD_DEMO_URL, RESOLVED_DEVICE_NAME, CLOUD_AGENT_KEY) if CLOUD_DEMO_URL and CLOUD_AGENT_KEY else None,
    material_roots=OMNIBULL_MATERIAL_ROOTS,
    dispatch_interval_seconds=OMNIBULL_PUBLISH_DISPATCH_INTERVAL_SECONDS,
)
openclaw_omnidrive_runtime_sync_thread = None
openclaw_omnidrive_runtime_sync_lock = threading.Lock()
openclaw_omnidrive_runtime_sync_stop = threading.Event()
openclaw_omnidrive_last_model_sync_at = 0.0
openclaw_omnidrive_last_model_sync_lock = threading.Lock()
openclaw_gateway_daily_reload_thread = None
openclaw_gateway_daily_reload_lock = threading.Lock()
openclaw_gateway_daily_reload_stop = threading.Event()
log_runtime_health(app_logger, OMNIBULL_RUNTIME_HEALTH)

# 限制上传文件大小为160MB
app.config['MAX_CONTENT_LENGTH'] = 160 * 1024 * 1024

# 获取当前目录（假设 index.html 和 assets 在这里）
current_dir = os.path.dirname(os.path.abspath(__file__))


def get_cors_allow_origin(origin):
    if not origin:
        return '*'
    if '*' in OMNIBULL_CORS_ALLOWED_ORIGINS:
        # When credentials are enabled we must echo the request origin instead of '*'.
        return origin if OMNIBULL_CORS_ALLOW_CREDENTIALS else '*'
    if origin in OMNIBULL_CORS_ALLOWED_ORIGINS:
        return origin
    return None


@app.before_request
def handle_cors_preflight():
    if request.method != 'OPTIONS':
        return None

    response = app.make_default_options_response()
    return apply_cors_headers(response)


@app.after_request
def apply_cors_headers(response):
    origin = request.headers.get('Origin')
    allow_origin = get_cors_allow_origin(origin)
    if allow_origin:
        requested_headers = request.headers.get('Access-Control-Request-Headers')
        allow_headers = requested_headers or ', '.join(OMNIBULL_CORS_ALLOWED_HEADERS)
        response.headers['Access-Control-Allow-Origin'] = allow_origin
        vary = response.headers.get('Vary')
        if vary:
            if 'Origin' not in vary:
                response.headers['Vary'] = f'{vary}, Origin'
        else:
            response.headers['Vary'] = 'Origin'
        response.headers['Access-Control-Allow-Methods'] = ', '.join(OMNIBULL_CORS_ALLOWED_METHODS)
        response.headers['Access-Control-Allow-Headers'] = allow_headers
        response.headers['Access-Control-Expose-Headers'] = ', '.join(OMNIBULL_CORS_EXPOSE_HEADERS)
        response.headers['Access-Control-Max-Age'] = str(OMNIBULL_CORS_MAX_AGE)
        if OMNIBULL_CORS_ALLOW_CREDENTIALS:
            response.headers['Access-Control-Allow-Credentials'] = 'true'
    return response


def serialize_account_row(row, status=None):
    row_status = row['status'] if status is None else status
    return [row['id'], row['type'], row['filePath'], row['userName'], row_status]


def serialize_account_detail(row, status=None):
    row_status = row['status'] if status is None else status
    cookie_path = get_cookie_dir() / row['filePath']
    platform_type = int(row['type'])
    cookie_exists = account_storage_exists(row)
    storage_backend = "database" if has_persisted_storage_state(row) else ("legacy_file" if cookie_path.exists() else "missing")
    return {
        "id": row['id'],
        "platformType": platform_type,
        "platformName": PLATFORM_LABELS.get(platform_type, "未知平台"),
        "filePath": row['filePath'],
        "cookieFilePath": row['filePath'],
        "cookieAbsolutePath": str(cookie_path.resolve()),
        "userName": row['userName'],
        "status": row_status,
        "cookieExists": cookie_exists,
        "storageBackend": storage_backend,
        "lastValidationAt": row["lastValidationAt"] if "lastValidationAt" in row.keys() else None,
        "lastValidationMessage": row["lastValidationMessage"] if "lastValidationMessage" in row.keys() else None,
    }


async def validate_account_rows(conn, rows):
    validated_rows = []
    updates = []

    for row in rows:
        result = await check_cookie_detail(row['type'], row['filePath'], headless=True)
        status = 1 if result.get("ok") else 0
        message = None if status == 1 else str(result.get("message") or "").strip() or "本地 cookie 当前不可用"
        validated_rows.append(serialize_account_row(row, status))
        updates.append((status, message, row['id']))

    if updates:
        cursor = conn.cursor()
        cursor.executemany(
            '''
            UPDATE user_info
            SET status = ?,
                lastValidationAt = CURRENT_TIMESTAMP,
                lastValidationMessage = ?
            WHERE id = ?
            ''',
            updates
        )
        conn.commit()

    return validated_rows


def get_request_payload():
    if request.is_json:
        return request.get_json(silent=True) or {}
    return request.values.to_dict()


def extract_skill_api_key():
    auth_header = str(request.headers.get("Authorization") or "").strip()
    if auth_header.lower().startswith("bearer "):
        return auth_header.split(" ", 1)[1].strip()
    return str(request.headers.get("X-Omnibull-Key") or "").strip()


def _is_loopback_request():
    normalized = str(request.remote_addr or "").strip()
    if not normalized:
        return False
    if normalized.startswith("[") and "]" in normalized:
        normalized = normalized[1:].split("]", 1)[0]
    elif normalized.count(":") == 1 and "." in normalized:
        normalized = normalized.rsplit(":", 1)[0]
    try:
        return ipaddress.ip_address(normalized).is_loopback
    except ValueError:
        return False


def ensure_skill_api_authorized():
    if not OMNIBULL_API_KEY:
        return None
    if _is_loopback_request():
        return None
    provided = extract_skill_api_key()
    if provided and secrets.compare_digest(provided, OMNIBULL_API_KEY):
        return None
    return jsonify({
        "code": 401,
        "msg": "OmniBull skill API 鉴权失败",
        "data": None,
    }), 401


def fetch_account_rows(account_ids=None):
    ensure_account_storage_schema()
    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        if account_ids:
            placeholders = ",".join("?" for _ in account_ids)
            cursor.execute(
                f'''
                SELECT * FROM user_info
                WHERE id IN ({placeholders})
                ORDER BY id DESC
                ''',
                [int(account_id) for account_id in account_ids],
            )
        else:
            cursor.execute(
                '''
                SELECT * FROM user_info
                ORDER BY id DESC
                '''
            )
        return cursor.fetchall()


def is_backend_session_alive(session):
    thread = (session or {}).get("thread")
    return bool(thread and thread.is_alive())


def serialize_backend_session(session):
    if not session:
        return None
    return {
        "accountId": session.get("accountId"),
        "platformType": session.get("platformType"),
        "accountName": session.get("accountName"),
        "startedAt": session.get("startedAt"),
        "status": "running" if is_backend_session_alive(session) else "closed",
    }


def get_active_backend_session(account_id):
    normalized_id = int(account_id)
    with backend_session_lock:
        existing = active_backend_sessions.get(normalized_id)
        if existing and is_backend_session_alive(existing):
            return dict(existing)
        if existing:
            active_backend_sessions.pop(normalized_id, None)
    return None


def clear_backend_session(account_id, thread=None):
    normalized_id = int(account_id)
    with backend_session_lock:
        existing = active_backend_sessions.get(normalized_id)
        if not existing:
            return
        if thread is not None and existing.get("thread") is not thread:
            return
        active_backend_sessions.pop(normalized_id, None)


def run_open_backend_session_thread(account_id, platform_type, account_name):
    current_thread = threading.current_thread()
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    try:
        loop.run_until_complete(
            open_platform_backend_session(
                int(platform_type),
                str(account_name).strip(),
                int(account_id),
                keep_window_open=True,
            )
        )
    except Exception as exc:
        login_logger.exception(
            "backend session thread failed account_id={} platform_type={} account_name={} error={}",
            account_id,
            platform_type,
            account_name,
            exc,
        )
    finally:
        try:
            loop.run_until_complete(loop.shutdown_asyncgens())
        except Exception:
            pass
        asyncio.set_event_loop(None)
        loop.close()
        clear_backend_session(account_id, thread=current_thread)


def start_backend_session(row):
    account_id = int(row["id"])
    with backend_session_lock:
        existing = active_backend_sessions.get(account_id)
        if existing and is_backend_session_alive(existing):
            return None, dict(existing)
        if existing:
            active_backend_sessions.pop(account_id, None)

        thread = threading.Thread(
            target=run_open_backend_session_thread,
            args=(account_id, int(row["type"]), str(row["userName"]).strip()),
            daemon=True,
        )
        session = {
            "accountId": account_id,
            "platformType": int(row["type"]),
            "accountName": str(row["userName"]).strip(),
            "startedAt": datetime.now(timezone.utc).isoformat(),
            "thread": thread,
        }
        active_backend_sessions[account_id] = session

    try:
        thread.start()
    except Exception:
        clear_backend_session(account_id, thread=thread)
        raise
    return dict(session), None


def handle_open_backend_request(account_id, *, require_skill_auth=False):
    if require_skill_auth:
        auth_error = ensure_skill_api_authorized()
        if auth_error:
            return auth_error

    rows = fetch_account_rows(account_ids=[account_id])
    if not rows:
        return jsonify({"code": 404, "msg": "账号不存在", "data": None}), 404

    row = rows[0]
    if not get_backend_entry_config(row["type"]):
        return jsonify({
            "code": 400,
            "msg": "当前账号平台暂不支持进入后台，首版仅支持抖音、视频号、快手",
            "data": None,
        }), 400

    existing_session = get_active_backend_session(account_id)
    if existing_session:
        return jsonify({
            "code": 409,
            "msg": "该账号后台已打开，请勿重复启动",
            "data": serialize_backend_session(existing_session),
        }), 409

    session, conflict = start_backend_session(row)
    if conflict:
        return jsonify({
            "code": 409,
            "msg": "该账号后台已打开，请勿重复启动",
            "data": serialize_backend_session(conflict),
        }), 409

    return jsonify({
        "code": 200,
        "msg": "已在本机打开后台",
        "data": serialize_backend_session(session),
    }), 200


def fetch_account_rows_by_file_paths(account_file_paths):
    ensure_account_storage_schema()
    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        placeholders = ",".join("?" for _ in account_file_paths)
        cursor.execute(
            f'''
            SELECT * FROM user_info
            WHERE filePath IN ({placeholders})
            ORDER BY id DESC
            ''',
            account_file_paths,
        )
        return cursor.fetchall()


def resolve_account_file_paths(account_ids=None, account_file_paths=None):
    account_file_paths = [str(value).strip() for value in (account_file_paths or []) if str(value).strip()]
    if account_file_paths:
        rows = fetch_account_rows_by_file_paths(account_file_paths)
        found_paths = {row["filePath"] for row in rows}
        missing = sorted(set(account_file_paths) - found_paths)
        if missing:
            raise ValueError(f"以下账号文件路径不存在: {missing}")
        return account_file_paths

    account_ids = [int(account_id) for account_id in (account_ids or [])]
    if not account_ids:
        raise ValueError("缺少账号信息")

    rows = fetch_account_rows(account_ids=account_ids)
    resolved = [row["filePath"] for row in rows]
    missing = sorted(set(account_ids) - {row["id"] for row in rows})
    if missing:
        raise ValueError(f"以下账号不存在: {missing}")
    return resolved


def resolve_skill_file_items(files):
    file_items = []
    for item in files or []:
        if isinstance(item, str):
            resolved = resolve_material_reference(OMNIBULL_MATERIAL_ROOTS, absolute_path=item)
        elif isinstance(item, dict):
            resolved = resolve_material_reference(
                OMNIBULL_MATERIAL_ROOTS,
                root_name=item.get("root") or item.get("materialRoot"),
                relative_path=item.get("path") or item.get("relativePath"),
                absolute_path=item.get("absolutePath"),
            )
        else:
            raise ValueError("files 中存在不支持的素材格式")

        absolute_path = Path(resolved["absolutePath"])
        if not absolute_path.exists():
            raise ValueError(f"素材不存在: {absolute_path}")
        if not absolute_path.is_file():
            raise ValueError(f"素材不是文件: {absolute_path}")

        file_items.append(
            {
                "root": resolved["rootName"],
                "path": resolved["relativePath"],
                "absolutePath": str(absolute_path),
            }
        )
    return file_items


def build_skill_status_payload():
    ensure_publish_task_manager_started()
    ensure_omnidrive_ai_task_manager_started()
    agent_status = cloud_agent.status() if cloud_agent else None
    omnidrive_agent_status = omnidrive_agent.status() if omnidrive_agent else None
    shared_runtime = _load_shared_agent_runtime_config(refresh=False) or {}
    omnidrive_auth_summary = get_omnidrive_authorization_summary()

    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()

        cursor.execute("SELECT COUNT(*) AS count FROM user_info")
        account_total = cursor.fetchone()["count"]

        cursor.execute(
            '''
            SELECT status, COUNT(*) AS count
            FROM user_info
            GROUP BY status
            '''
        )
        account_statuses = {
            str(row["status"]): row["count"]
            for row in cursor.fetchall()
        }

        publish_task_counts = {}
        try:
            cursor.execute(
                '''
                SELECT status, COUNT(*) AS count
                FROM publish_tasks
                GROUP BY status
                '''
            )
            publish_task_counts = {
                row["status"]: row["count"]
                for row in cursor.fetchall()
            }
        except sqlite3.OperationalError:
            publish_task_counts = {}

    return {
        "deviceName": RESOLVED_DEVICE_NAME,
        "deviceCode": DEVICE_CODE,
        "deviceIdentitySource": DEVICE_IDENTITY.get("source"),
        "deviceIdentityPath": DEVICE_IDENTITY.get("path"),
        "runtimeHealth": OMNIBULL_RUNTIME_HEALTH,
        "materialRoots": list_material_roots(OMNIBULL_MATERIAL_ROOTS),
        "skillApiAuthEnabled": bool(OMNIBULL_API_KEY),
        "cloudAgentConfig": get_cloud_agent_config(),
        "cloudAgent": agent_status,
        "omniDriveAgentConfig": get_omnidrive_agent_config(),
        "omniDriveAgent": omnidrive_agent_status,
        "omnidriveAuthorized": omnidrive_auth_summary["omnidriveAuthorized"],
        "omnidriveAuthState": omnidrive_auth_summary["authState"],
        "omnidriveAuthReason": omnidrive_auth_summary["reason"],
        "boundUser": omnidrive_auth_summary["boundUser"],
        "boundDevice": omnidrive_auth_summary["boundDevice"],
        "availableSkills": omnidrive_auth_summary["availableSkills"],
        "hermesBridgeConfig": get_hermes_bridge_config(),
        "sharedAgentRuntime": _sanitize_shared_runtime_config_for_response(shared_runtime) if shared_runtime else None,
        "accounts": {
            "total": account_total,
            "byStatus": account_statuses,
        },
        "publishTasks": {
            "byStatus": publish_task_counts,
        },
        "aiTasks": omnidrive_ai_task_manager.summary(),
    }


def relay_remote_login_status(status_queue, bridge):
    try:
        login_logger.debug("remote login relay started session_id={}", bridge.session_id)
        bridge.push_log("本地浏览器已启动，等待二维码...")

        while True:
            msg = status_queue.get(timeout=240)

            if msg == "200":
                login_logger.info("remote login relay success session_id={}", bridge.session_id)
                bridge.push_login_success()
                break

            if msg == "500":
                login_logger.warning("remote login relay failed session_id={}", bridge.session_id)
                bridge.push_login_failed()
                break

            if isinstance(msg, dict):
                event_type = msg.get("type")
                payload = msg.get("payload") or {}

                if event_type == "qr_ready" and payload.get("qrData"):
                    login_logger.info("remote login relay qr ready session_id={}", bridge.session_id)
                    bridge.push_qr(payload["qrData"])
                    bridge.push_log(payload.get("message") or "二维码已就绪，请在远端页面扫码")
                    continue

                if event_type == "verification_required":
                    login_logger.warning("remote login relay verification required session_id={}", bridge.session_id)
                    bridge.push_verification(payload)
                    if payload.get("message"):
                        bridge.push_log(payload["message"])
                    continue

                if event_type == "login_failed":
                    message = str(payload.get("message") or "").strip() or "本地登录失败"
                    login_logger.warning("remote login relay login failed session_id={} message={}", bridge.session_id, message)
                    bridge.push_login_failed(message)
                    break

                if event_type == "log" and payload.get("message"):
                    bridge.push_log(payload["message"])
                    continue

            if isinstance(msg, str) and msg:
                login_logger.info("remote login relay qr text forwarded session_id={}", bridge.session_id)
                bridge.push_qr(msg)
                bridge.push_log("二维码已就绪，请在远端页面扫码")
    except Empty:
        login_logger.warning("remote login relay timed out session_id={}", bridge.session_id)
        try:
            bridge.push_login_failed("本地登录超时，未等到扫码完成")
        except Exception:
            pass
    except Exception as exc:
        login_logger.exception("remote login relay failed session_id={} error={}", bridge.session_id, exc)
        try:
            bridge.push_login_failed(f"远端同步失败: {exc}")
        except Exception:
            pass


def get_cloud_agent_config():
    blocked_reason = None
    if not CLOUD_AGENT_ENABLED:
        blocked_reason = "CLOUD_AGENT_ENABLED 未开启"
    elif not CLOUD_DEMO_URL:
        blocked_reason = "CLOUD_DEMO_URL 未配置"
    elif not CLOUD_AGENT_KEY:
        blocked_reason = "CLOUD_AGENT_KEY 未配置"

    return {
        "enabled": CLOUD_AGENT_ENABLED,
        "cloudUrl": CLOUD_DEMO_URL,
        "deviceName": RESOLVED_DEVICE_NAME,
        "deviceCode": DEVICE_CODE,
        "agentKeyConfigured": bool(CLOUD_AGENT_KEY),
        "pollInterval": CLOUD_AGENT_POLL_INTERVAL,
        "heartbeatInterval": CLOUD_AGENT_HEARTBEAT_INTERVAL,
        "startEligible": blocked_reason is None,
        "blockedReason": blocked_reason
    }


def get_omnidrive_agent_config():
    blocked_reason = None
    if not OMNIDRIVE_AGENT_ENABLED:
        blocked_reason = "OMNIDRIVE_AGENT_ENABLED 未开启"
    elif not OMNIDRIVE_BASE_URL:
        blocked_reason = "OMNIDRIVE_BASE_URL 未配置"
    elif not OMNIDRIVE_AGENT_KEY:
        blocked_reason = "OMNIDRIVE_AGENT_KEY 未配置"

    return {
        "enabled": OMNIDRIVE_AGENT_ENABLED,
        "cloudUrl": OMNIDRIVE_BASE_URL,
        "deviceName": RESOLVED_DEVICE_NAME,
        "deviceCode": DEVICE_CODE,
        "agentKeyConfigured": bool(OMNIDRIVE_AGENT_KEY),
        "pollInterval": OMNIDRIVE_AGENT_POLL_INTERVAL,
        "aiPollInterval": OMNIDRIVE_AGENT_AI_POLL_INTERVAL,
        "heartbeatInterval": OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL,
        "accountSyncInterval": OMNIDRIVE_ACCOUNT_SYNC_INTERVAL,
        "materialSyncInterval": OMNIDRIVE_MATERIAL_SYNC_INTERVAL,
        "skillSyncInterval": OMNIDRIVE_SKILL_SYNC_INTERVAL,
        "publishSyncInterval": OMNIDRIVE_PUBLISH_SYNC_INTERVAL,
        "maxMaterialFiles": OMNIDRIVE_MATERIAL_SYNC_MAX_FILES,
        "startEligible": blocked_reason is None,
        "blockedReason": blocked_reason,
    }


def get_hermes_bridge_config():
    return {
        "apiBaseUrl": HERMES_API_SERVER_BASE_URL,
        "apiKeyConfigured": bool(HERMES_API_SERVER_KEY),
        "timeoutSeconds": HERMES_API_SERVER_TIMEOUT,
        "profile": HERMES_PROFILE_NAME,
        "sharedRuntimeConfigPath": str(HERMES_SHARED_RUNTIME_CONFIG_PATH),
        "localBridgeClientPath": str(HERMES_LOCAL_BRIDGE_CLIENT_PATH),
        "routingMode": "shared_runtime_single_source",
    }


def _sync_platform_capabilities_from_session_payload(payload):
    try:
        return cache_platform_capabilities_from_session_payload(payload)
    except Exception as exc:
        app_logger.warning("sync platform capabilities from OmniDrive session failed error={}", exc)
        return get_visible_platform_capabilities()


def refresh_platform_capabilities_from_omnidrive():
    if not OMNIDRIVE_AGENT_ENABLED or not OMNIDRIVE_BASE_URL or not OMNIDRIVE_AGENT_KEY:
        return get_visible_platform_capabilities()

    try:
        status_code, payload = fetch_omnidrive_device_session()
    except Exception as exc:
        app_logger.warning("refresh platform capabilities from OmniDrive failed error={}", exc)
        return get_visible_platform_capabilities()

    if status_code >= 400:
        return get_visible_platform_capabilities()
    return _sync_platform_capabilities_from_session_payload(payload)


def fetch_omnidrive_device_session():
    if not OMNIDRIVE_BASE_URL:
        raise RuntimeError("OMNIDRIVE_BASE_URL 未配置")
    if not OMNIDRIVE_AGENT_KEY:
        raise RuntimeError("OMNIDRIVE_AGENT_KEY 未配置")

    endpoint = f"{OMNIDRIVE_BASE_URL.rstrip('/')}/api/v1/agent/device-session/{DEVICE_CODE}"
    req = urllib_request.Request(
        endpoint,
        method='GET',
        headers={
            "Accept": "application/json",
            "X-Agent-Key": OMNIDRIVE_AGENT_KEY,
        },
    )
    try:
        with urllib_request.urlopen(req, timeout=15) as response:
            payload = response.read().decode("utf-8")
            parsed = json.loads(payload) if payload else {}
            if isinstance(parsed, dict):
                parsed.setdefault("apiBaseUrl", OMNIDRIVE_BASE_URL)
                parsed.setdefault("cloudUrl", parsed.get("apiBaseUrl") or OMNIDRIVE_BASE_URL)
                _sync_platform_capabilities_from_session_payload(parsed)
            return response.status, parsed
    except urllib_error.HTTPError as exc:
        payload = exc.read().decode("utf-8")
        try:
            parsed = json.loads(payload) if payload else {}
        except json.JSONDecodeError:
            parsed = {"error": payload or str(exc)}
        return exc.code, parsed


def _resolve_omnidrive_api_base_url(api_base_url=""):
    resolved = str(api_base_url or OMNIDRIVE_BASE_URL or "").strip().rstrip("/")
    if not resolved:
        raise RuntimeError("OMNIDRIVE_BASE_URL 未配置")
    return resolved


def omnidrive_cloud_json_request(method, path, access_token=None, payload=None, timeout=60, query=None, api_base_url=""):
    base_url = _resolve_omnidrive_api_base_url(api_base_url)

    endpoint = f"{base_url}{path}"
    if query:
        cleaned = {key: value for key, value in (query or {}).items() if value not in (None, "")}
        if cleaned:
            endpoint = f"{endpoint}?{urlencode(cleaned)}"

    headers = {
        "Accept": "application/json",
    }
    data = None
    if access_token:
        headers["Authorization"] = f"Bearer {access_token}"
    if payload is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(payload).encode("utf-8")

    req = urllib_request.Request(
        endpoint,
        method=method,
        headers=headers,
        data=data,
    )
    try:
        with urllib_request.urlopen(req, timeout=timeout) as response:
            raw_payload = response.read().decode("utf-8")
            return response.status, json.loads(raw_payload) if raw_payload else {}
    except urllib_error.HTTPError as exc:
        raw_payload = exc.read().decode("utf-8")
        try:
            parsed = json.loads(raw_payload) if raw_payload else {}
        except json.JSONDecodeError:
            parsed = {"error": raw_payload or str(exc)}
        return exc.code, parsed


def get_omnidrive_device_session_data():
    status_code, payload = fetch_omnidrive_device_session()
    if status_code >= 400:
        message = ""
        if isinstance(payload, dict):
            message = str(payload.get("error") or payload.get("message") or payload.get("msg") or "").strip()
        raise RuntimeError(message or "获取 OmniDrive 设备会话失败")

    if not isinstance(payload, dict):
        raise RuntimeError("OmniDrive 设备会话响应格式无效")

    access_token = str(payload.get("accessToken") or "").strip()
    if not access_token:
        raise RuntimeError("OmniDrive 设备会话缺少 accessToken")

    device = payload.get("device") if isinstance(payload.get("device"), dict) else {}
    device_id = str(device.get("id") or "").strip()
    if not device_id:
        raise RuntimeError("OmniDrive 设备会话缺少设备信息")

    return {
        "accessToken": access_token,
        "device": device,
        "user": payload.get("user") if isinstance(payload.get("user"), dict) else {},
        "source": str(payload.get("source") or "").strip() or "agent_device_session",
        "expiresAt": payload.get("expiresAt"),
        "apiBaseUrl": str(payload.get("apiBaseUrl") or payload.get("cloudUrl") or OMNIDRIVE_BASE_URL or "").strip(),
    }


def _summarize_bound_session_user(user):
    if not isinstance(user, dict):
        return None
    user_id = str(user.get("id") or "").strip()
    name = str(user.get("name") or "").strip()
    email = str(user.get("email") or "").strip()
    phone = str(user.get("phone") or "").strip()
    if not any((user_id, name, email, phone)):
        return None
    return {
        "id": user_id or None,
        "name": name or None,
        "email": email or None,
        "phone": phone or None,
    }


def _summarize_bound_session_device(device):
    if not isinstance(device, dict):
        return None
    device_id = str(device.get("id") or "").strip()
    device_code = str(device.get("deviceCode") or "").strip()
    name = str(device.get("name") or "").strip()
    if not any((device_id, device_code, name)):
        return None
    return {
        "id": device_id or None,
        "deviceCode": device_code or None,
        "name": name or None,
        "isEnabled": device.get("isEnabled"),
        "defaultChatModel": str(device.get("defaultChatModel") or "").strip() or None,
        "defaultImageModel": str(device.get("defaultImageModel") or "").strip() or None,
        "defaultVideoModel": str(device.get("defaultVideoModel") or "").strip() or None,
    }


def _build_omnidrive_authorization_summary(session_payload=None, reason=""):
    authorized = isinstance(session_payload, dict) and bool(str(session_payload.get("accessToken") or "").strip())
    clean_reason = str(reason or "").strip()
    if authorized:
        clean_reason = ""
    return {
        "authState": "authorized" if authorized else "blocked",
        "reason": clean_reason,
        "omnidriveAuthorized": authorized,
        "boundUser": _summarize_bound_session_user(session_payload.get("user")) if isinstance(session_payload, dict) else None,
        "boundDevice": _summarize_bound_session_device(session_payload.get("device")) if isinstance(session_payload, dict) else None,
        "availableSkills": list(OPENCLAW_OMNIDRIVE_AVAILABLE_SKILLS if authorized else []),
    }


def get_omnidrive_authorization_summary():
    try:
        status_code, payload = fetch_omnidrive_device_session()
    except Exception as exc:
        return _build_omnidrive_authorization_summary(reason=str(exc))

    if status_code >= 400:
        message = ""
        if isinstance(payload, dict):
            message = str(payload.get("error") or payload.get("message") or payload.get("msg") or "").strip()
        return _build_omnidrive_authorization_summary(reason=message or "获取 OmniDrive 设备会话失败")
    return _build_omnidrive_authorization_summary(payload)


def _ensure_correlation_id(value=None):
    normalized = str(value or "").strip()
    return normalized or uuid.uuid4().hex


def _resolve_source_category(value, fallback):
    normalized = str(value or "").strip()
    return normalized or fallback


def _write_json_file(path, payload):
    target = Path(path).expanduser()
    target.parent.mkdir(parents=True, exist_ok=True)
    serialized = json.dumps(payload, ensure_ascii=False, indent=2) + "\n"
    with target.open("w", encoding="utf-8") as handle:
        handle.write(serialized)
    return str(target)


def _read_json_file(path):
    target = Path(path).expanduser()
    if not target.exists():
        return None
    try:
        with target.open("r", encoding="utf-8") as handle:
            return json.load(handle)
    except Exception:
        return None


def _sanitize_shared_runtime_config_for_response(payload):
    sanitized = json.loads(json.dumps(payload or {}, ensure_ascii=False))
    provider = sanitized.get("provider") if isinstance(sanitized.get("provider"), dict) else None
    if provider is not None:
        if provider.get("apiKey"):
            provider["apiKey"] = "[REDACTED]"
        if provider.get("accessToken"):
            provider["accessToken"] = "[REDACTED]"
    return sanitized


def _build_shared_agent_runtime_config(model_items=None, api_base_url="", access_token="", device=None):
    device = device if isinstance(device, dict) else {}
    provider_base_url = _resolve_openclaw_omnidrive_provider_base_url(api_base_url)
    provider_api_key = str(OMNIBULL_API_KEY or "").strip() or None
    models = _normalize_openclaw_omnidrive_models(model_items)
    payload = {
        "version": HERMES_SHARED_RUNTIME_VERSION,
        "updatedAt": datetime.now(timezone.utc).isoformat(),
        "device": {
            "deviceCode": DEVICE_CODE,
            "deviceName": RESOLVED_DEVICE_NAME,
            "boundDeviceId": str(device.get("id") or "").strip() or None,
            "boundDeviceCode": str(device.get("deviceCode") or "").strip() or None,
            "boundDeviceName": str(device.get("name") or "").strip() or None,
        },
        "provider": {
            "name": "omnidrive",
            "format": "openai-compatible",
            "baseUrl": provider_base_url,
        },
        "defaults": {
            "chatModel": str(device.get("defaultChatModel") or "").strip() or None,
            "imageModel": str(device.get("defaultImageModel") or "").strip() or None,
            "videoModel": str(device.get("defaultVideoModel") or "").strip() or None,
        },
        "routing": {
            "mode": "shared_runtime_single_source",
            "chat": "omnidrive_dynamic",
            "image": "omnidrive_dynamic",
            "video": "omnidrive_dynamic",
            "fallbackChat": "omnidrive_openai_proxy",
        },
        "openclaw": {
            "configPaths": [str(path) for path in OPENCLAW_OMNIDRIVE_CONFIG_PATHS],
        },
        "hermes": {
            "profile": HERMES_PROFILE_NAME,
            "apiBaseUrl": HERMES_API_SERVER_BASE_URL,
            "sharedConfigPath": str(HERMES_SHARED_RUNTIME_CONFIG_PATH),
            "localBridgeClientPath": str(HERMES_LOCAL_BRIDGE_CLIENT_PATH),
        },
        "models": models,
    }
    if provider_api_key:
        payload["provider"]["apiKey"] = provider_api_key
    return payload


def sync_hermes_shared_runtime_config(model_items=None, api_base_url="", access_token="", device=None):
    payload = _build_shared_agent_runtime_config(
        model_items=model_items,
        api_base_url=api_base_url,
        access_token=access_token,
        device=device,
    )
    path = _write_json_file(HERMES_SHARED_RUNTIME_CONFIG_PATH, payload)
    return payload, [path]


def _load_shared_agent_runtime_config(refresh=False):
    if refresh:
        try:
            result = refresh_openclaw_omnidrive_runtime_config(include_models=True)
            return result.get("sharedRuntime")
        except Exception:
            pass
    return _read_json_file(HERMES_SHARED_RUNTIME_CONFIG_PATH)


def hermes_api_json_request(method, path, payload=None, timeout=None, query=None, include_auth=True):
    base_url = str(HERMES_API_SERVER_BASE_URL or "").strip().rstrip("/")
    if not base_url:
        raise RuntimeError("HERMES_API_SERVER_BASE_URL 未配置")

    endpoint = f"{base_url}{path}"
    if query:
        cleaned = {key: value for key, value in (query or {}).items() if value not in (None, "")}
        if cleaned:
            endpoint = f"{endpoint}?{urlencode(cleaned)}"

    headers = {
        "Accept": "application/json",
    }
    if include_auth and HERMES_API_SERVER_KEY:
        headers["Authorization"] = f"Bearer {HERMES_API_SERVER_KEY}"

    data = None
    if payload is not None:
        headers["Content-Type"] = "application/json"
        data = json.dumps(payload).encode("utf-8")

    req = urllib_request.Request(
        endpoint,
        method=method,
        headers=headers,
        data=data,
    )
    try:
        with urllib_request.urlopen(req, timeout=timeout or HERMES_API_SERVER_TIMEOUT) as response:
            raw_payload = response.read().decode("utf-8")
            return response.status, json.loads(raw_payload) if raw_payload else {}
    except urllib_error.HTTPError as exc:
        raw_payload = exc.read().decode("utf-8")
        try:
            parsed = json.loads(raw_payload) if raw_payload else {}
        except json.JSONDecodeError:
            parsed = {"error": raw_payload or str(exc)}
        return exc.code, parsed


def _extract_hermes_response_text(payload):
    if not isinstance(payload, dict):
        return ""
    output = payload.get("output") if isinstance(payload.get("output"), list) else []
    parts = []
    for item in output:
        if not isinstance(item, dict):
            continue
        if str(item.get("type") or "").strip() != "message":
            continue
        if str(item.get("role") or "").strip() != "assistant":
            continue
        content = item.get("content") if isinstance(item.get("content"), list) else []
        for content_item in content:
            if not isinstance(content_item, dict):
                continue
            if str(content_item.get("type") or "").strip() != "output_text":
                continue
            text = str(content_item.get("text") or "").strip()
            if text:
                parts.append(text)
    return "\n".join(parts).strip()


def _extract_hermes_chat_completion_text(payload):
    if not isinstance(payload, dict):
        return ""
    choices = payload.get("choices") if isinstance(payload.get("choices"), list) else []
    for choice in choices:
        if not isinstance(choice, dict):
            continue
        message = choice.get("message") if isinstance(choice.get("message"), dict) else {}
        text = message.get("content")
        if isinstance(text, str) and text.strip():
            return text.strip()
    return ""


def _resolve_openclaw_omnidrive_provider_base_url(api_base_url):
    return f"http://127.0.0.1:{SAU_BACKEND_PORT}{OMNIDRIVE_OPENAI_PROXY_BASE_PATH}"


def _normalize_openclaw_omnidrive_models(models):
    normalized = []
    seen = set()

    for item in models or []:
        if isinstance(item, dict):
            model_id = str(item.get("modelName") or item.get("name") or item.get("id") or "").strip()
        else:
            model_id = str(item or "").strip()
        if not model_id or model_id in seen:
            continue
        seen.add(model_id)
        normalized.append(model_id)

    return normalized


def _mark_openclaw_omnidrive_model_sync(now=None):
    global openclaw_omnidrive_last_model_sync_at
    with openclaw_omnidrive_last_model_sync_lock:
        openclaw_omnidrive_last_model_sync_at = float(now if now is not None else time.time())


def _should_refresh_openclaw_omnidrive_model_cache(now=None):
    current = float(now if now is not None else time.time())
    with openclaw_omnidrive_last_model_sync_lock:
        last_sync_at = float(openclaw_omnidrive_last_model_sync_at or 0.0)
    return last_sync_at <= 0 or (current - last_sync_at) >= OPENCLAW_OMNIDRIVE_MODEL_SYNC_INTERVAL_SECONDS


def _iter_openai_model_aliases(value):
    cleaned = str(value or "").strip()
    if not cleaned:
        return []

    variants = []
    seen = set()

    def add(candidate):
        candidate = str(candidate or "").strip()
        if not candidate or candidate in seen:
            return
        seen.add(candidate)
        variants.append(candidate)

    add(cleaned)
    if "/" in cleaned:
        add(cleaned.split("/", 1)[1].strip())

    queue = list(variants)
    while queue:
        candidate = queue.pop(0)
        derived = [
            candidate.replace(".", "-"),
            candidate.replace("-", "."),
            candidate.replace("_", "-"),
            candidate.replace("_", "."),
        ]
        for item in derived:
            if item and item not in seen:
                add(item)
                queue.append(item)

    return variants


def _build_openclaw_omnidrive_model_entry(model_id, include_api=False):
    entry = {
        "id": model_id,
        "name": OPENCLAW_OMNIDRIVE_MODEL_NAME_OVERRIDES.get(model_id) or model_id,
        "reasoning": False,
        "input": ["text", "image"] if model_id in OPENCLAW_OMNIDRIVE_MULTIMODAL_MODELS else ["text"],
        "cost": {
            "input": 0,
            "output": 0,
            "cacheRead": 0,
            "cacheWrite": 0,
        },
        "contextWindow": 128000,
        "maxTokens": 8192,
    }
    if include_api:
        entry["api"] = "openai-completions"
    return entry


def _build_openclaw_omnidrive_default_alias(model_id, default_chat_model="", used_aliases=None):
    model_id = str(model_id or "").strip()
    if not model_id:
        return ""

    if model_id == str(default_chat_model or "").strip():
        alias = "omni"
    else:
        alias = {
            "gemini-3.1-pro-preview": "omni-gemini",
            "gpt-5.4": "omni-gpt",
            "qwen3.5-plus": "omni-qwen",
        }.get(model_id)
        if not alias:
            slug = re.sub(r"[^a-z0-9]+", "-", model_id.lower()).strip("-")
            alias = f"omni-{slug or 'model'}"

    if isinstance(used_aliases, set):
        base_alias = alias
        suffix = 2
        while alias in used_aliases:
            alias = f"{base_alias}-{suffix}"
            suffix += 1
        used_aliases.add(alias)

    return alias


def _initialize_openclaw_omnidrive_provider(data, *, is_root_config):
    container = (data.setdefault("models", {}) if is_root_config else data)
    providers = container.setdefault("providers", {})
    provider = providers.get("omnidrive")
    if not isinstance(provider, dict):
        provider = {}
        providers["omnidrive"] = provider
    provider.setdefault("api", "openai-completions")
    if not isinstance(provider.get("models"), list):
        provider["models"] = []
    return provider


def _write_openclaw_omnidrive_config_if_changed(path, serialized):
    existing = None
    if path.exists():
        try:
            existing = path.read_text(encoding="utf-8")
        except Exception as exc:
            app_logger.warning("failed to read OpenClaw config before write path={} error={}", path, exc)

    if existing == serialized:
        return False

    with path.open("w", encoding="utf-8") as handle:
        handle.write(serialized)
    return True


def sync_openclaw_omnidrive_model_configs(models, api_base_url="", access_token="", default_chat_model=""):
    normalized_ids = _normalize_openclaw_omnidrive_models(models)
    models_supplied = models is not None
    provider_base_url = _resolve_openclaw_omnidrive_provider_base_url(api_base_url)
    provider_api_key = str(OMNIBULL_API_KEY or "").strip()
    default_chat_model = str(default_chat_model or "").strip()
    changed_paths = []

    for path in OPENCLAW_OMNIDRIVE_CONFIG_PATHS:
        is_root_config = path.name == "openclaw.json"
        if path.exists():
            try:
                with path.open("r", encoding="utf-8") as handle:
                    data = json.load(handle)
            except Exception as exc:
                app_logger.warning("skip OpenClaw model sync because config cannot be read path={} error={}", path, exc)
                continue
        elif not is_root_config:
            data = {}
            try:
                path.parent.mkdir(parents=True, exist_ok=True)
            except Exception as exc:
                app_logger.warning("skip OpenClaw model sync because config parent cannot be created path={} error={}", path, exc)
                continue
        else:
            continue

        provider = _initialize_openclaw_omnidrive_provider(data, is_root_config=is_root_config)

        if provider_base_url:
            provider["baseUrl"] = provider_base_url
        if provider_api_key:
            provider["apiKey"] = provider_api_key
        else:
            provider.pop("apiKey", None)

        if models_supplied:
            provider["models"] = [
                _build_openclaw_omnidrive_model_entry(model_id, include_api=True)
                for model_id in normalized_ids
            ]

        if is_root_config:
            defaults_config = data.setdefault("agents", {}).setdefault("defaults", {})
            defaults = defaults_config.get("models")
            if not isinstance(defaults, dict):
                defaults = {}
                defaults_config["models"] = defaults
            if models_supplied:
                for key in list(defaults.keys()):
                    if str(key).startswith("omnidrive/"):
                        defaults.pop(key, None)

                omni_target_model = default_chat_model or (normalized_ids[0] if normalized_ids else "")
                alias_model_ids = list(normalized_ids)
                if omni_target_model and omni_target_model not in alias_model_ids:
                    alias_model_ids.insert(0, omni_target_model)

                used_aliases = set()
                for model_id in alias_model_ids:
                    alias = _build_openclaw_omnidrive_default_alias(
                        model_id,
                        default_chat_model=omni_target_model,
                        used_aliases=used_aliases,
                    )
                    if alias:
                        defaults[f"omnidrive/{model_id}"] = {"alias": alias}
            elif default_chat_model:
                for key, value in list(defaults.items()):
                    alias = value.get("alias") if isinstance(value, dict) else None
                    if str(key).startswith("omnidrive/") and alias == "omni":
                        defaults.pop(key, None)
                defaults[f"omnidrive/{default_chat_model}"] = {"alias": "omni"}

        serialized = json.dumps(data, ensure_ascii=False, indent=2) + "\n"
        try:
            if _write_openclaw_omnidrive_config_if_changed(path, serialized):
                changed_paths.append(str(path))
        except Exception as exc:
            app_logger.warning("failed to write OpenClaw model sync config path={} error={}", path, exc)

    return changed_paths


def _extract_omnidrive_chat_models(payload):
    models = []
    for item in payload or []:
        if not isinstance(item, dict):
            continue
        model_id = str(item.get("modelName") or item.get("name") or item.get("id") or "").strip()
        if model_id:
            models.append(item)
    return models


def _build_omnidrive_chat_model_alias_map(payload):
    alias_map = {}
    for item in payload or []:
        if not isinstance(item, dict):
            continue
        canonical = str(item.get("modelName") or item.get("name") or item.get("id") or "").strip()
        if not canonical:
            continue
        for alias in (
            canonical,
            str(item.get("id") or "").strip(),
            str(item.get("name") or "").strip(),
        ):
            for alias_variant in _iter_openai_model_aliases(alias):
                alias_map[alias_variant] = canonical
    alias_map["default-chat"] = alias_map.get("default-chat") or alias_map.get("gemini-3.1-pro-preview") or "gemini-3.1-pro-preview"
    alias_map["default"] = alias_map["default-chat"]
    alias_map["omnidrive-default-chat"] = alias_map["default-chat"]
    return alias_map


def _resolve_requested_omnidrive_chat_model_name(raw_model_name, default_model_name, access_token, api_base_url=""):
    requested = _normalize_openai_model_name(raw_model_name, default_model_name)
    try:
        status_code, payload = omnidrive_cloud_json_request(
            "GET",
            "/api/v1/ai/models",
            access_token=access_token,
            query={"category": "chat"},
            timeout=20,
            api_base_url=api_base_url,
        )
    except Exception:
        return requested

    if status_code >= 400:
        return requested

    alias_map = _build_omnidrive_chat_model_alias_map(payload)
    return alias_map.get(requested, requested)


def sync_openclaw_omnidrive_models_from_cloud():
    result = refresh_openclaw_omnidrive_runtime_config(include_models=True)
    return result["changedPaths"]


def refresh_openclaw_omnidrive_runtime_config(include_models=False):
    session = get_omnidrive_device_session_data()
    model_items = None
    if include_models:
        status_code, payload = omnidrive_cloud_json_request(
            "GET",
            "/api/v1/ai/models",
            access_token=session["accessToken"],
            query={"category": "chat"},
            timeout=30,
            api_base_url=session.get("apiBaseUrl"),
        )
        if status_code >= 400:
            message = ""
            if isinstance(payload, dict):
                message = str(payload.get("error") or payload.get("message") or payload.get("msg") or "").strip()
            raise RuntimeError(message or "列出 OmniDrive 聊天模型失败")
        model_items = _extract_omnidrive_chat_models(payload)
        _mark_openclaw_omnidrive_model_sync()

    changed_paths = sync_openclaw_omnidrive_model_configs(
        model_items,
        api_base_url=session.get("apiBaseUrl"),
        access_token=session.get("accessToken"),
        default_chat_model=session.get("device", {}).get("defaultChatModel"),
    )
    if include_models and changed_paths:
        _reload_openclaw_gateway_after_omnidrive_model_sync(
            changed_paths,
            reason="refresh_openclaw_omnidrive_runtime_config",
        )
    shared_runtime, shared_runtime_paths = sync_hermes_shared_runtime_config(
        model_items,
        api_base_url=session.get("apiBaseUrl"),
        access_token=session.get("accessToken"),
        device=session.get("device"),
    )
    return {
        "session": session,
        "modelItems": model_items,
        "changedPaths": changed_paths,
        "sharedRuntime": shared_runtime,
        "sharedRuntimePaths": shared_runtime_paths,
    }


def _parse_omnidrive_expiry_epoch(expires_at):
    if not expires_at:
        return None
    if isinstance(expires_at, (int, float)):
        return float(expires_at)

    text = str(expires_at or "").strip()
    if not text:
        return None
    if text.endswith("Z"):
        text = f"{text[:-1]}+00:00"

    try:
        parsed = datetime.fromisoformat(text)
    except ValueError:
        return None

    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.timestamp()


def ensure_openclaw_omnidrive_models_synced():
    try:
        changed_paths = sync_openclaw_omnidrive_models_from_cloud()
        if changed_paths:
            app_logger.info("synced OpenClaw OmniDrive model configs paths={}", ",".join(changed_paths))
    except Exception as exc:
        app_logger.warning("sync OpenClaw OmniDrive model configs skipped error={}", exc)


def _openclaw_omnidrive_runtime_sync_loop():
    next_model_sync_at = 0.0
    while not openclaw_omnidrive_runtime_sync_stop.is_set():
        sleep_seconds = OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS
        include_models = time.time() >= next_model_sync_at

        try:
            result = refresh_openclaw_omnidrive_runtime_config(include_models=include_models)
            expires_epoch = _parse_omnidrive_expiry_epoch(result["session"].get("expiresAt"))
            if include_models:
                next_model_sync_at = time.time() + OPENCLAW_OMNIDRIVE_MODEL_SYNC_INTERVAL_SECONDS
            if expires_epoch:
                seconds_until_expiry = expires_epoch - time.time()
                if seconds_until_expiry > 0:
                    refresh_after = seconds_until_expiry - OPENCLAW_OMNIDRIVE_TOKEN_REFRESH_MARGIN_SECONDS
                    sleep_seconds = min(sleep_seconds, refresh_after)
        except Exception as exc:
            log_throttled(
                app_logger,
                "WARNING",
                "openclaw_omnidrive_runtime_sync:error",
                OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS,
                "OpenClaw OmniDrive runtime sync failed interval={} error={}",
                OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS,
                exc,
            )

        openclaw_omnidrive_runtime_sync_stop.wait(max(60, int(sleep_seconds)))


def ensure_openclaw_omnidrive_runtime_sync_started():
    global openclaw_omnidrive_runtime_sync_thread

    if not OMNIDRIVE_AGENT_ENABLED or not OMNIDRIVE_BASE_URL or not OMNIDRIVE_AGENT_KEY:
        return
    if not any(path.exists() for path in OPENCLAW_OMNIDRIVE_CONFIG_PATHS):
        return

    with openclaw_omnidrive_runtime_sync_lock:
        if (
            openclaw_omnidrive_runtime_sync_thread is not None
            and openclaw_omnidrive_runtime_sync_thread.is_alive()
        ):
            return

        openclaw_omnidrive_runtime_sync_stop.clear()
        openclaw_omnidrive_runtime_sync_thread = threading.Thread(
            target=_openclaw_omnidrive_runtime_sync_loop,
            name="openclaw-omnidrive-runtime-sync",
            daemon=True,
        )
        openclaw_omnidrive_runtime_sync_thread.start()
        app_logger.info(
            "started OpenClaw OmniDrive runtime sync interval={} model_sync_interval={} refresh_margin={}",
            OPENCLAW_OMNIDRIVE_RUNTIME_SYNC_INTERVAL_SECONDS,
            OPENCLAW_OMNIDRIVE_MODEL_SYNC_INTERVAL_SECONDS,
            OPENCLAW_OMNIDRIVE_TOKEN_REFRESH_MARGIN_SECONDS,
        )


def _compute_next_openclaw_gateway_daily_reload_epoch(now=None):
    current = now or datetime.now()
    if isinstance(current, (int, float)):
        current = datetime.fromtimestamp(float(current))
    scheduled = current.replace(
        hour=OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR,
        minute=OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE,
        second=0,
        microsecond=0,
    )
    if scheduled <= current:
        scheduled += timedelta(days=1)
    return scheduled.timestamp()


def _restart_openclaw_gateway_for_config_reload():
    command = OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND
    try:
        completed = subprocess.run(
            ["/bin/bash", "-lc", command],
            capture_output=True,
            text=True,
            timeout=OPENCLAW_GATEWAY_DAILY_RELOAD_TIMEOUT_SECONDS,
            check=False,
        )
    except Exception as exc:
        return {
            "ok": False,
            "command": command,
            "error": str(exc),
            "returncode": None,
            "stdout": "",
            "stderr": "",
        }
    return {
        "ok": completed.returncode == 0,
        "command": command,
        "error": "",
        "returncode": completed.returncode,
        "stdout": str(completed.stdout or "").strip(),
        "stderr": str(completed.stderr or "").strip(),
    }


def _attempt_openclaw_gateway_daily_reload():
    ensure_publish_task_manager_started()
    running_tasks = publish_task_manager.list_tasks(
        limit=max(1, int(getattr(publish_task_manager, "worker_count", 1))),
        status="running",
    )
    if running_tasks:
        return {
            "status": "deferred",
            "delaySeconds": OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS,
            "runningTaskCount": len(running_tasks),
            "taskUuids": [str(task.get("taskUuid") or "") for task in running_tasks if str(task.get("taskUuid") or "").strip()],
        }

    restart_result = _restart_openclaw_gateway_for_config_reload()
    if restart_result.get("ok"):
        return {
            "status": "restarted",
            "delaySeconds": 0,
            "command": restart_result.get("command"),
        }
    return {
        "status": "retry",
        "delaySeconds": OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS,
        "command": restart_result.get("command"),
        "returncode": restart_result.get("returncode"),
        "error": restart_result.get("error"),
        "stdout": restart_result.get("stdout"),
        "stderr": restart_result.get("stderr"),
    }


def _reload_openclaw_gateway_after_omnidrive_model_sync(changed_paths, reason="omnidrive_model_sync"):
    normalized_paths = [str(path).strip() for path in (changed_paths or []) if str(path).strip()]
    if not normalized_paths or not OPENCLAW_OMNIDRIVE_RELOAD_GATEWAY_ON_MODEL_SYNC:
        return None

    result = _attempt_openclaw_gateway_daily_reload()
    status = str((result or {}).get("status") or "").strip()
    joined_paths = ",".join(normalized_paths)

    if status == "restarted":
        app_logger.info(
            "reloaded OpenClaw gateway after OmniDrive model sync reason={} paths={} command={}",
            reason,
            joined_paths,
            result.get("command") or OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND,
        )
    elif status == "deferred":
        app_logger.info(
            "deferred OpenClaw gateway reload after OmniDrive model sync reason={} paths={} delay_seconds={} running_task_count={}",
            reason,
            joined_paths,
            result.get("delaySeconds"),
            result.get("runningTaskCount"),
        )
    elif status:
        app_logger.warning(
            "OpenClaw gateway reload after OmniDrive model sync failed reason={} paths={} status={} returncode={} stderr={}",
            reason,
            joined_paths,
            status,
            result.get("returncode"),
            result.get("stderr") or result.get("error") or "",
        )
    return result


def _openclaw_gateway_daily_reload_loop():
    next_reload_at = _compute_next_openclaw_gateway_daily_reload_epoch()
    app_logger.info(
        "started OpenClaw gateway daily config reload schedule={} defer_seconds={} command={} next_reload_at={}",
        f"{OPENCLAW_GATEWAY_DAILY_RELOAD_HOUR:02d}:{OPENCLAW_GATEWAY_DAILY_RELOAD_MINUTE:02d}",
        OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS,
        OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND,
        datetime.fromtimestamp(next_reload_at).isoformat(timespec="seconds"),
    )

    while not openclaw_gateway_daily_reload_stop.is_set():
        now = time.time()
        wait_seconds = max(30, int(min(300, max(0, next_reload_at - now))))
        if openclaw_gateway_daily_reload_stop.wait(wait_seconds):
            return

        now = time.time()
        if now < next_reload_at:
            continue

        try:
            result = _attempt_openclaw_gateway_daily_reload()
        except Exception as exc:
            result = {
                "status": "retry",
                "delaySeconds": OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS,
                "error": str(exc),
            }

        status = str(result.get("status") or "").strip()
        if status == "restarted":
            next_reload_at = _compute_next_openclaw_gateway_daily_reload_epoch()
            app_logger.info(
                "OpenClaw gateway daily config reload succeeded next_reload_at={}",
                datetime.fromtimestamp(next_reload_at).isoformat(timespec="seconds"),
            )
            continue

        delay_seconds = max(60, int(result.get("delaySeconds") or OPENCLAW_GATEWAY_DAILY_RELOAD_DEFER_SECONDS))
        next_reload_at = time.time() + delay_seconds
        if status == "deferred":
            app_logger.info(
                "deferred OpenClaw gateway daily config reload running_publish_tasks={} next_retry_at={} task_uuids={}",
                int(result.get("runningTaskCount") or 0),
                datetime.fromtimestamp(next_reload_at).isoformat(timespec="seconds"),
                ",".join(result.get("taskUuids") or []) or "-",
            )
        else:
            app_logger.warning(
                "OpenClaw gateway daily config reload failed command={} returncode={} error={} next_retry_at={} stdout={} stderr={}",
                result.get("command") or OPENCLAW_GATEWAY_DAILY_RELOAD_COMMAND,
                result.get("returncode"),
                result.get("error") or "-",
                datetime.fromtimestamp(next_reload_at).isoformat(timespec="seconds"),
                _compact_log_text(result.get("stdout"), 300),
                _compact_log_text(result.get("stderr"), 300),
            )


def ensure_openclaw_gateway_daily_reload_started():
    global openclaw_gateway_daily_reload_thread

    if not OPENCLAW_GATEWAY_DAILY_RELOAD_ENABLED:
        return
    if not any(path.exists() for path in OPENCLAW_OMNIDRIVE_CONFIG_PATHS):
        return

    with openclaw_gateway_daily_reload_lock:
        if (
            openclaw_gateway_daily_reload_thread is not None
            and openclaw_gateway_daily_reload_thread.is_alive()
        ):
            return

        openclaw_gateway_daily_reload_stop.clear()
        openclaw_gateway_daily_reload_thread = threading.Thread(
            target=_openclaw_gateway_daily_reload_loop,
            name="openclaw-gateway-daily-reload",
            daemon=True,
        )
        openclaw_gateway_daily_reload_thread.start()


def _flatten_openai_message_content(content):
    if isinstance(content, str):
        return content.strip()
    if isinstance(content, list):
        parts = []
        for item in content:
            if not isinstance(item, dict):
                continue
            if str(item.get("type") or "").strip() != "text":
                continue
            text = item.get("text")
            if isinstance(text, str) and text.strip():
                parts.append(text.strip())
        return "\n".join(parts).strip()
    return ""


def _strip_openclaw_sender_metadata(text):
    if not isinstance(text, str):
        return ""

    cleaned = text.strip()
    if not cleaned:
        return ""

    for _ in range(3):
        updated = re.sub(
            r"^\s*Sender \(untrusted metadata\):\s*```json\s*.*?```\s*",
            "",
            cleaned,
            flags=re.DOTALL,
        )
        updated = re.sub(r"^\s*\[[^\]]+\]\s*", "", updated)
        updated = updated.strip()
        if updated == cleaned:
            break
        cleaned = updated

    cleaned = re.sub(r"\n{3,}", "\n\n", cleaned)
    return cleaned.strip()


def _extract_last_openai_user_prompt(messages):
    for item in reversed(messages or []):
        if not isinstance(item, dict):
            continue
        if str(item.get("role") or "").strip() != "user":
            continue
        text = _strip_openclaw_sender_metadata(_flatten_openai_message_content(item.get("content")))
        if text:
            return text
    return ""


def _extract_openai_function_tools(tools):
    result = {}
    for item in tools or []:
        if not isinstance(item, dict):
            continue
        if str(item.get("type") or "function").strip() != "function":
            continue
        function_def = item.get("function") if isinstance(item.get("function"), dict) else {}
        name = str(function_def.get("name") or "").strip()
        if not name:
            continue
        result[name] = function_def
    return result


def _index_openai_tool_calls(messages):
    result = {}
    for item in messages or []:
        if not isinstance(item, dict):
            continue
        if str(item.get("role") or "").strip() != "assistant":
            continue
        for tool_call in item.get("tool_calls") or []:
            if not isinstance(tool_call, dict):
                continue
            tool_call_id = str(tool_call.get("id") or "").strip()
            function_payload = tool_call.get("function") if isinstance(tool_call.get("function"), dict) else {}
            tool_name = str(function_payload.get("name") or "").strip()
            if tool_call_id and tool_name:
                result[tool_call_id] = {
                    "toolName": tool_name,
                    "toolCall": tool_call,
                }
    return result


def _extract_last_openai_tool_result(messages):
    tool_call_index = _index_openai_tool_calls(messages)
    for item in reversed(messages or []):
        if not isinstance(item, dict):
            continue
        role = str(item.get("role") or "").strip()
        if role not in {"tool", "toolResult"}:
            continue
        tool_call_id = str(item.get("tool_call_id") or "").strip()
        indexed = tool_call_index.get(tool_call_id) if tool_call_id else None
        tool_name = str(item.get("tool_name") or "").strip() or (indexed or {}).get("toolName") or ""
        content = item.get("content")
        flattened = _flatten_openai_message_content(content)
        if not flattened and isinstance(content, str):
            flattened = content.strip()
        return {
            "toolName": tool_name,
            "toolCallId": tool_call_id,
            "content": flattened,
            "rawContent": content,
        }
    return None


def _safe_json_loads(text):
    if not isinstance(text, str):
        return None
    text = text.strip()
    if not text:
        return None
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def _extract_unsupported_openai_tool_names(tool_map):
    unsupported = []
    for name in tool_map:
        if name not in OMNIDRIVE_OPENAI_SUPPORTED_TOOL_NAMES:
            unsupported.append(name)
    return sorted(unsupported)


def _strip_openclaw_tooling_sections(text):
    if not isinstance(text, str):
        return ""
    cleaned = re.sub(r"(^|\n)## Tooling\b.*?(?=\n##\s|\Z)", "\n", text, flags=re.DOTALL)
    cleaned = re.sub(r"\n{3,}", "\n\n", cleaned)
    return cleaned.strip()


def _is_openai_bridge_error_text(text):
    normalized = str(text or "").strip()
    if not normalized:
        return False
    error_markers = (
        "[Gemini Error:",
        "MALFORMED_FUNCTION_CALL",
        "UNEXPECTED_TOOL_CALL",
        "Malformed function call:",
    )
    return any(marker in normalized for marker in error_markers)


def _prompt_requests_local_system_action(prompt):
    normalized = str(prompt or "").strip().lower()
    if not normalized:
        return False

    explicit_local_tool_phrases = (
        "read local",
        "read file",
        "write file",
        "edit file",
        "edit config",
        "run command",
        "run shell",
        "restart service",
        "reload service",
        "list local skills",
        "web_search",
        "web search",
        "读取本地文件",
        "读取文件",
        "修改文件",
        "修改配置",
        "编辑配置",
        "执行命令",
        "执行终端命令",
        "运行命令",
        "重启服务",
        "重新加载服务",
        "列出本地技能",
        "查看本地技能",
        "网页搜索",
        "联网搜索",
    )
    explicit_tool_names = ("read", "write", "edit", "exec", "process", "web_search")
    if any(phrase in normalized for phrase in explicit_local_tool_phrases):
        return True
    return any(
        re.search(rf"(^|[^a-z_]){re.escape(name)}([^a-z_]|$)", normalized)
        for name in explicit_tool_names
    )


def _sanitize_omnidrive_forwarded_openai_messages(messages, unsupported_tool_names):
    sanitized = []

    for item in messages or []:
        if not isinstance(item, dict):
            continue

        role = str(item.get("role") or "").strip()
        if role in {"tool", "toolResult"}:
            # OmniDrive upstream chat currently does not understand OpenAI tool
            # messages; media tool results are handled before sanitization.
            continue
        if role not in {"system", "user", "assistant"}:
            continue

        text = _flatten_openai_message_content(item.get("content"))
        if role == "system":
            text = _strip_openclaw_tooling_sections(text)
        else:
            text = _strip_openclaw_sender_metadata(str(text or "").strip())

        if role == "assistant" and _is_openai_bridge_error_text(text):
            continue

        if not text:
            continue

        sanitized.append({
            "role": role,
            "content": text,
        })

    if unsupported_tool_names:
        sanitized.insert(0, {
            "role": "system",
            "content": OMNIDRIVE_OPENAI_LOCAL_TOOL_SYSTEM_GUARD,
        })

    return sanitized


def _prompt_is_media_capability_question(prompt):
    normalized = str(prompt or "").strip().lower()
    if not normalized:
        return False

    media_keywords = ("视频", "video", "veo", "图片", "图像", "image", "海报", "封面", "壁纸", "做图", "画图", "混剪", "剪辑", "二创")
    capability_keywords = (
        "你能",
        "你可以",
        "能不能",
        "可不可以",
        "能否",
        "是否可以",
        "可以帮我",
        "能帮我",
        "can you",
        "could you",
        "are you able",
    )
    generic_generation_keywords = ("做一个", "做个", "生成一个", "生成个", "create a", "make a", "generate a")

    if any(keyword in normalized for keyword in media_keywords) and any(
        keyword in normalized for keyword in capability_keywords
    ):
        return True

    return (
        any(keyword in normalized for keyword in media_keywords)
        and any(keyword in normalized for keyword in generic_generation_keywords)
        and normalized.endswith(("吗", "吗？", "吗?", "?", "？"))
        and len(normalized) <= 36
    )


def _select_openai_media_tool(prompt, tool_map):
    normalized = str(prompt or "").strip().lower()
    if not normalized:
        return None

    skip_keywords = ("脚本", "文案", "提示词", "prompt", "教程", "方案", "流程", "步骤")
    if any(keyword in normalized for keyword in skip_keywords):
        return None

    if "omnidrive_mix_video" in tool_map and any(keyword in normalized for keyword in ("混剪", "剪辑", "二创", "混剪视频")):
        return {
            "toolName": "omnidrive_mix_video",
            "arguments": {
                "action": "create",
                "scriptText": str(prompt or "").strip(),
            },
        }

    if "omnidrive_video" in tool_map and any(keyword in normalized for keyword in ("veo", "视频", "video", "短片")):
        return {
            "toolName": "omnidrive_video",
            "arguments": {
                "prompt": str(prompt or "").strip(),
                "wait": False,
            },
        }

    if "omnidrive_image" in tool_map and any(
        keyword in normalized
        for keyword in ("图片", "图像", "image", "海报", "封面", "壁纸", "插画", "配图", "做图", "画图")
    ):
        return {
            "toolName": "omnidrive_image",
            "arguments": {
                "prompt": str(prompt or "").strip(),
                "wait": True,
            },
        }

    return None


def _build_openai_tool_call(tool_name, arguments):
    return {
        "id": f"call_{uuid.uuid4().hex[:24]}",
        "type": "function",
        "function": {
            "name": tool_name,
            "arguments": json.dumps(arguments or {}, ensure_ascii=False),
        },
    }


def _build_media_clarification_text(prompt, tool_map):
    normalized = str(prompt or "").strip().lower()
    if "omnidrive_mix_video" in tool_map and any(keyword in normalized for keyword in ("混剪", "剪辑", "二创", "混剪视频")):
        return (
            "可以，我能帮你发起 OmniDrive 混剪任务。"
            "\n请直接告诉我混剪脚本文案，并附上源视频和参考音频；"
            "\n如果你已经绑定了发布账号，也可以一并告诉我要发布到哪个账号和发布时间。"
        )
    if "omnidrive_video" in tool_map and any(keyword in normalized for keyword in ("veo", "视频", "video", "短片")):
        return (
            "可以，我能帮你发起 OmniDrive 视频生成。"
            "\n请直接告诉我你想生成的视频内容，比如主体、风格、镜头、时长或比例；"
            "\n例如：用 veo 生成一个 8 秒的城市夜景延时视频，16:9，电影感。"
        )
    if "omnidrive_image" in tool_map and any(
        keyword in normalized
        for keyword in ("图片", "图像", "image", "海报", "封面", "壁纸", "插画", "配图", "做图", "画图")
    ):
        return (
            "可以，我能帮你发起 OmniDrive 图片生成。"
            "\n请直接告诉我图片主体、风格、比例或用途；"
            "\n例如：生成一张赛博朋克风格的产品海报，竖版 9:16。"
        )
    return ""


def _collect_omnidrive_media_result_details(payload):
    parsed = payload if isinstance(payload, dict) else {}
    job = parsed.get("job") if isinstance(parsed.get("job"), dict) else {}
    workspace = parsed.get("workspace") if isinstance(parsed.get("workspace"), dict) else {}

    artifact_items = []
    for source in (workspace.get("artifacts"), parsed.get("artifacts")):
        if not isinstance(source, list):
            continue
        for item in source:
            if not isinstance(item, dict):
                continue
            artifact_items.append(item)

    public_urls = []
    seen_urls = set()
    for source in (workspace.get("publicUrls"), parsed.get("publicUrls")):
        if not isinstance(source, list):
            continue
        for item in source:
            url = str(item or "").strip()
            if url and url not in seen_urls:
                seen_urls.add(url)
                public_urls.append(url)

    artifacts = []
    for item in artifact_items:
        public_url = str(item.get("publicUrl") or item.get("url") or "").strip()
        if public_url and public_url not in seen_urls:
            seen_urls.add(public_url)
            public_urls.append(public_url)
        artifacts.append(
            {
                "artifactType": str(item.get("artifactType") or "").strip(),
                "mimeType": str(item.get("mimeType") or "").strip(),
                "fileName": str(item.get("fileName") or item.get("title") or "").strip(),
                "publicUrl": public_url,
                "textContent": str(item.get("textContent") or "").strip(),
            }
        )

    return {
        "jobId": str(job.get("id") or workspace.get("jobId") or "").strip(),
        "modelName": str(job.get("modelName") or workspace.get("modelName") or "").strip(),
        "status": str(workspace.get("status") or job.get("status") or "").strip(),
        "message": str(workspace.get("message") or parsed.get("message") or "").strip(),
        "text": str(workspace.get("text") or "").strip(),
        "nextStep": str(parsed.get("nextStep") or "").strip(),
        "publicUrls": public_urls,
        "artifacts": artifacts,
    }


def _summarize_openai_media_tool_result(tool_name, content):
    parsed = _safe_json_loads(content)
    if not isinstance(parsed, dict):
        return str(content or "").strip() or "工具执行完成。"

    if tool_name == "omnidrive_mix_video":
        lines = ["混剪任务已处理。"]
        if parsed.get("id"):
            lines.append(f"任务 ID：{parsed['id']}")
        if parsed.get("status"):
            lines.append(f"状态：{parsed['status']}")
        result_asset = parsed.get("resultAsset") if isinstance(parsed.get("resultAsset"), dict) else {}
        if result_asset.get("publicUrl"):
            lines.append("成片地址：")
            lines.append(str(result_asset.get("publicUrl")).strip())
        if parsed.get("platform") or parsed.get("accountName"):
            lines.append(f"发布目标：{str(parsed.get('platform') or '').strip()} {str(parsed.get('accountName') or '').strip()}".strip())
        return "\n".join(lines)

    noun = "视频" if tool_name == "omnidrive_video" else "图片"
    details = _collect_omnidrive_media_result_details(parsed)

    lines = []
    if details["publicUrls"]:
        lines.append(f"{noun}已生成完成。")
    elif details["jobId"]:
        lines.append(f"{noun}任务已提交。")
    else:
        lines.append(f"{noun}请求已处理。")

    if details["jobId"]:
        lines.append(f"任务 ID：{details['jobId']}")
    if details["modelName"]:
        lines.append(f"模型：{details['modelName']}")
    if details["status"]:
        lines.append(f"状态：{details['status']}")
    if details["publicUrls"]:
        lines.append("结果地址：")
        lines.extend(details["publicUrls"][:3])
    elif details["nextStep"]:
        lines.append(details["nextStep"])
    if details["artifacts"]:
        lines.append("结果明细：")
        for artifact in details["artifacts"][:5]:
            artifact_type = artifact["artifactType"] or "artifact"
            file_name = artifact["fileName"] or "unnamed"
            line = f"- {artifact_type} {file_name}"
            if artifact["mimeType"]:
                line += f" ({artifact['mimeType']})"
            if artifact["publicUrl"]:
                line += f": {artifact['publicUrl']}"
            elif artifact["textContent"]:
                line += f": {artifact['textContent'][:120]}"
            lines.append(line)
    if details["message"]:
        lines.append(f"消息：{details['message']}")
    if details["text"]:
        lines.append(details["text"])

    return "\n".join(lines)


def _extract_workspace_text(workspace):
    if not isinstance(workspace, dict):
        return ""

    job = workspace.get("job") if isinstance(workspace.get("job"), dict) else {}
    output_payload = job.get("outputPayload") if isinstance(job.get("outputPayload"), dict) else {}
    text = output_payload.get("text")
    if isinstance(text, str) and text.strip():
        return text.strip()

    for artifact in workspace.get("artifacts") or []:
        if not isinstance(artifact, dict):
            continue
        if str(artifact.get("artifactType") or "").strip() != "text":
            continue
        text_content = artifact.get("textContent")
        if isinstance(text_content, str) and text_content.strip():
            return text_content.strip()

    return ""


def _normalize_openai_model_name(raw_model_name, fallback_model_name):
    model_name = str(raw_model_name or "").strip()
    if not model_name:
        return str(fallback_model_name or "").strip()
    if "/" in model_name:
        return model_name.split("/", 1)[1].strip()
    if model_name in {"default-chat", "default", "omnidrive-default-chat"}:
        return str(fallback_model_name or "").strip()
    return model_name


def _openai_error_response(message, status_code=400, error_type="invalid_request_error"):
    return jsonify({
        "error": {
            "message": str(message or "request failed"),
            "type": error_type,
        }
    }), status_code


def _build_openai_usage():
    return {
        "prompt_tokens": 0,
        "completion_tokens": 0,
        "total_tokens": 0,
    }


def _build_openai_chat_completion_response(job_id, model_name, text):
    created = int(time.time())
    response_id = f"chatcmpl-{job_id}"
    return {
        "id": response_id,
        "object": "chat.completion",
        "created": created,
        "model": model_name,
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": text,
                },
                "finish_reason": "stop",
            }
        ],
        "usage": _build_openai_usage(),
    }


def _build_openai_tool_call_completion_response(job_id, model_name, tool_call):
    created = int(time.time())
    response_id = f"chatcmpl-{job_id}"
    return {
        "id": response_id,
        "object": "chat.completion",
        "created": created,
        "model": model_name,
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": "",
                    "tool_calls": [tool_call],
                },
                "finish_reason": "tool_calls",
            }
        ],
        "usage": _build_openai_usage(),
    }


def _build_openai_chat_stream_chunks(job_id, model_name, text=None, tool_call=None):
    created = int(time.time())
    response_id = f"chatcmpl-{job_id}"
    if tool_call:
        return [
            {
                "id": response_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model_name,
                "choices": [
                    {
                        "index": 0,
                        "delta": {"role": "assistant"},
                        "finish_reason": None,
                    }
                ],
            },
            {
                "id": response_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model_name,
                "choices": [
                    {
                        "index": 0,
                        "delta": {"tool_calls": [{**tool_call, "index": 0}]},
                        "finish_reason": None,
                    }
                ],
            },
            {
                "id": response_id,
                "object": "chat.completion.chunk",
                "created": created,
                "model": model_name,
                "choices": [
                    {
                        "index": 0,
                        "delta": {},
                        "finish_reason": "tool_calls",
                    }
                ],
            },
        ]
    return [
        {
            "id": response_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": model_name,
            "choices": [
                {
                    "index": 0,
                    "delta": {"role": "assistant"},
                    "finish_reason": None,
                }
            ],
        },
        {
            "id": response_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": model_name,
            "choices": [
                {
                    "index": 0,
                    "delta": {"content": text},
                    "finish_reason": None,
                }
            ],
        },
        {
            "id": response_id,
            "object": "chat.completion.chunk",
            "created": created,
            "model": model_name,
            "choices": [
                {
                    "index": 0,
                    "delta": {},
                    "finish_reason": "stop",
                }
            ],
        },
    ]


def _build_openai_chat_stream_chunk(response_id, model_name, delta=None, finish_reason=None):
    return {
        "id": response_id,
        "object": "chat.completion.chunk",
        "created": int(time.time()),
        "model": model_name,
        "choices": [
            {
                "index": 0,
                "delta": delta or {},
                "finish_reason": finish_reason,
            }
        ],
    }


def _prepare_omnidrive_openai_chat_context(openai_payload):
    session = get_omnidrive_device_session_data()
    access_token = session["accessToken"]
    api_base_url = session.get("apiBaseUrl")
    device = session["device"]
    device_id = str(device.get("id") or "").strip()
    default_model_name = str(device.get("defaultChatModel") or "").strip() or "gemini-3.1-pro-preview"
    model_name = _resolve_requested_omnidrive_chat_model_name(
        openai_payload.get("model"),
        default_model_name,
        access_token,
        api_base_url=api_base_url,
    )

    messages = openai_payload.get("messages")
    if not isinstance(messages, list) or not messages:
        raise ValueError("messages 不能为空")

    tool_result = _extract_last_openai_tool_result(messages)
    if tool_result and tool_result["toolName"] in {"omnidrive_image", "omnidrive_video", "omnidrive_mix_video"}:
        return {
            "accessToken": access_token,
            "apiBaseUrl": api_base_url,
            "deviceId": device_id,
            "messages": messages,
            "modelName": model_name,
            "specialCompletion": {
                "jobId": f"tool-result-{uuid.uuid4().hex}",
                "modelName": model_name,
                "text": _summarize_openai_media_tool_result(tool_result["toolName"], tool_result["content"]),
            },
        }

    prompt = _extract_last_openai_user_prompt(messages)
    if not prompt:
        raise ValueError("messages 中缺少可用的 user 文本内容")

    tool_map = _extract_openai_function_tools(openai_payload.get("tools"))
    unsupported_tool_names = _extract_unsupported_openai_tool_names(tool_map)
    clarification_text = _build_media_clarification_text(prompt, tool_map)
    if clarification_text and _prompt_is_media_capability_question(prompt):
        return {
            "accessToken": access_token,
            "apiBaseUrl": api_base_url,
            "deviceId": device_id,
            "messages": messages,
            "prompt": prompt,
            "modelName": model_name,
            "specialCompletion": {
                "jobId": f"tool-clarify-{uuid.uuid4().hex}",
                "modelName": model_name,
                "text": clarification_text,
            },
        }

    media_tool = _select_openai_media_tool(prompt, tool_map)
    if media_tool:
        return {
            "accessToken": access_token,
            "apiBaseUrl": api_base_url,
            "deviceId": device_id,
            "messages": messages,
            "prompt": prompt,
            "modelName": model_name,
            "specialCompletion": {
                "jobId": f"tool-call-{uuid.uuid4().hex}",
                "modelName": model_name,
                "toolCall": _build_openai_tool_call(media_tool["toolName"], media_tool["arguments"]),
            },
        }

    if unsupported_tool_names and _prompt_requests_local_system_action(prompt):
        return {
            "accessToken": access_token,
            "apiBaseUrl": api_base_url,
            "deviceId": device_id,
            "messages": messages,
            "prompt": prompt,
            "modelName": model_name,
            "specialCompletion": {
                "jobId": f"tool-guard-{uuid.uuid4().hex}",
                "modelName": model_name,
                "text": OMNIDRIVE_OPENAI_LOCAL_TOOL_GUARD_TEXT,
            },
        }

    sanitized_messages = _sanitize_omnidrive_forwarded_openai_messages(messages, unsupported_tool_names)
    sanitized_prompt = _extract_last_openai_user_prompt(sanitized_messages) or prompt
    if not sanitized_messages:
        sanitized_messages = [{"role": "user", "content": prompt}]

    return {
        "accessToken": access_token,
        "apiBaseUrl": api_base_url,
        "deviceId": device_id,
        "messages": sanitized_messages,
        "prompt": sanitized_prompt,
        "modelName": model_name,
        "specialCompletion": None,
    }


def _build_omnidrive_openai_chat_request_payload(openai_payload, context):
    request_payload = {
        # Use the cloud-executable source so current OmniDrive workers can pick up
        # OpenClaw main-chat requests without waiting for a cloud-side rollout.
        "source": "omnidrive_cloud",
        "jobType": "chat",
        "modelName": context["modelName"],
        "prompt": context["prompt"],
        "deviceId": context["deviceId"],
        "inputPayload": {
            "messages": context["messages"],
            "requestSource": "openclaw_main_chat",
        },
    }
    if openai_payload.get("temperature") is not None:
        request_payload["inputPayload"]["temperature"] = openai_payload.get("temperature")
    if openai_payload.get("max_tokens") is not None:
        request_payload["inputPayload"]["maxTokens"] = openai_payload.get("max_tokens")
    return request_payload


def _iter_sse_events(stream):
    event_name = None
    data_lines = []

    for raw_line in stream:
        line = raw_line.decode("utf-8", errors="replace").rstrip("\r\n")
        if line == "":
            if event_name is not None or data_lines:
                yield event_name or "message", "\n".join(data_lines)
            event_name = None
            data_lines = []
            continue
        if line.startswith(":"):
            continue
        if line.startswith("event:"):
            event_name = line.split(":", 1)[1].strip()
            continue
        if line.startswith("data:"):
            data_lines.append(line.split(":", 1)[1].lstrip())

    if event_name is not None or data_lines:
        yield event_name or "message", "\n".join(data_lines)


def _stream_special_openai_completion(special_completion):
    def event_stream():
        chunks = _build_openai_chat_stream_chunks(
            special_completion["jobId"],
            special_completion["modelName"],
            text=special_completion.get("text"),
            tool_call=special_completion.get("toolCall"),
        )
        for chunk in chunks:
            yield f"data: {json.dumps(chunk, ensure_ascii=False)}\n\n"
        yield "data: [DONE]\n\n"

    response = Response(event_stream(), mimetype="text/event-stream")
    response.headers["Cache-Control"] = "no-cache, no-transform"
    response.headers["Connection"] = "keep-alive"
    response.headers["X-Accel-Buffering"] = "no"
    return response


def stream_omnidrive_openai_chat_completion(openai_payload):
    context = _prepare_omnidrive_openai_chat_context(openai_payload)
    special_completion = context.get("specialCompletion")
    if special_completion:
        return _stream_special_openai_completion(special_completion)

    request_payload = _build_omnidrive_openai_chat_request_payload(openai_payload, context)
    endpoint = f"{_resolve_omnidrive_api_base_url(context.get('apiBaseUrl'))}/api/v1/ai/chat/stream"
    headers = {
        "Accept": "text/event-stream",
        "Cache-Control": "no-cache",
        "Content-Type": "application/json",
        "Authorization": f"Bearer {context['accessToken']}",
    }
    data = json.dumps(request_payload).encode("utf-8")

    def event_stream():
        upstream = None
        response_id = None
        role_sent = False
        emitted_text = ""
        model_name = context["modelName"]
        try:
            request_obj = urllib_request.Request(
                endpoint,
                method="POST",
                headers=headers,
                data=data,
            )
            upstream = urllib_request.urlopen(request_obj, timeout=300)

            for event_name, payload_text in _iter_sse_events(upstream):
                payload = _safe_json_loads(payload_text) if payload_text.strip() != "[DONE]" else None

                if event_name == "meta" and isinstance(payload, dict):
                    job_id = str(payload.get("jobId") or "").strip() or f"stream-{uuid.uuid4().hex}"
                    response_id = f"chatcmpl-{job_id}"
                    model_name = str(payload.get("modelName") or model_name).strip() or model_name
                    if not role_sent:
                        role = str(payload.get("role") or "assistant").strip() or "assistant"
                        yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={'role': role}), ensure_ascii=False)}\n\n"
                        role_sent = True
                    continue

                if event_name == "progress":
                    continue

                if event_name == "error":
                    message = ""
                    if isinstance(payload, dict):
                        message = str(payload.get("error") or payload.get("message") or "").strip()
                    yield f"data: {json.dumps({'error': {'message': message or 'OmniDrive 流式聊天失败', 'type': 'server_error'}}, ensure_ascii=False)}\n\n"
                    yield "data: [DONE]\n\n"
                    return

                if event_name == "delta" and isinstance(payload, dict):
                    if not response_id:
                        job_id = str(payload.get("jobId") or "").strip() or f"stream-{uuid.uuid4().hex}"
                        response_id = f"chatcmpl-{job_id}"
                    model_name = str(payload.get("modelName") or model_name).strip() or model_name
                    if not role_sent:
                        role = str(payload.get("role") or "assistant").strip() or "assistant"
                        yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={'role': role}), ensure_ascii=False)}\n\n"
                        role_sent = True
                    delta = str(payload.get("delta") or "")
                    if delta:
                        emitted_text += delta
                        yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={'content': delta}), ensure_ascii=False)}\n\n"
                    continue

                if event_name == "done" and isinstance(payload, dict):
                    if not response_id:
                        job_id = str(payload.get("jobId") or "").strip() or f"stream-{uuid.uuid4().hex}"
                        response_id = f"chatcmpl-{job_id}"
                    model_name = str(payload.get("modelName") or model_name).strip() or model_name
                    if not role_sent:
                        role = str(payload.get("role") or "assistant").strip() or "assistant"
                        yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={'role': role}), ensure_ascii=False)}\n\n"
                        role_sent = True
                    full_text = str(payload.get("text") or "")
                    if full_text.startswith(emitted_text):
                        remainder = full_text[len(emitted_text):]
                        if remainder:
                            yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={'content': remainder}), ensure_ascii=False)}\n\n"
                    finish_reason = str(payload.get("finishReason") or "").strip() or "stop"
                    yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={}, finish_reason=finish_reason), ensure_ascii=False)}\n\n"
                    yield "data: [DONE]\n\n"
                    return

            if response_id:
                yield f"data: {json.dumps(_build_openai_chat_stream_chunk(response_id, model_name, delta={}, finish_reason='stop'), ensure_ascii=False)}\n\n"
            else:
                yield f"data: {json.dumps({'error': {'message': 'OmniDrive 未返回任何流式事件', 'type': 'server_error'}}, ensure_ascii=False)}\n\n"
            yield "data: [DONE]\n\n"
        except urllib_error.HTTPError as exc:
            raw_payload = exc.read().decode("utf-8", errors="replace")
            parsed = _safe_json_loads(raw_payload)
            message = ""
            if isinstance(parsed, dict):
                message = str(parsed.get("error") or parsed.get("message") or "").strip()
            yield f"data: {json.dumps({'error': {'message': message or raw_payload or str(exc), 'type': 'server_error'}}, ensure_ascii=False)}\n\n"
            yield "data: [DONE]\n\n"
        except Exception as exc:
            yield f"data: {json.dumps({'error': {'message': str(exc), 'type': 'server_error'}}, ensure_ascii=False)}\n\n"
            yield "data: [DONE]\n\n"
        finally:
            if upstream is not None:
                upstream.close()

    response = Response(event_stream(), mimetype="text/event-stream")
    response.headers["Cache-Control"] = "no-cache, no-transform"
    response.headers["Connection"] = "keep-alive"
    response.headers["X-Accel-Buffering"] = "no"
    return response


def create_omnidrive_openai_chat_completion(openai_payload):
    context = _prepare_omnidrive_openai_chat_context(openai_payload)
    special_completion = context.get("specialCompletion")
    if special_completion:
        return special_completion

    access_token = context["accessToken"]
    model_name = context["modelName"]
    request_payload = _build_omnidrive_openai_chat_request_payload(openai_payload, context)

    status_code, create_payload = omnidrive_cloud_json_request(
        "POST",
        "/api/v1/ai/jobs",
        access_token=access_token,
        payload=request_payload,
        timeout=60,
        api_base_url=context.get("apiBaseUrl"),
    )
    if status_code >= 400:
        message = ""
        if isinstance(create_payload, dict):
            message = str(create_payload.get("error") or create_payload.get("message") or "").strip()
        raise RuntimeError(message or "创建 OmniDrive chat 任务失败")

    if not isinstance(create_payload, dict):
        raise RuntimeError("OmniDrive chat 任务响应格式无效")
    job_id = str(create_payload.get("id") or "").strip()
    if not job_id:
        raise RuntimeError("OmniDrive chat 任务缺少 job id")

    workspace = {}
    started_at = time.time()
    while True:
        status_code, workspace = omnidrive_cloud_json_request(
            "GET",
            f"/api/v1/ai/jobs/{job_id}/workspace",
            access_token=access_token,
            timeout=60,
            api_base_url=context.get("apiBaseUrl"),
        )
        if status_code >= 400:
            message = ""
            if isinstance(workspace, dict):
                message = str(workspace.get("error") or workspace.get("message") or "").strip()
            raise RuntimeError(message or "查询 OmniDrive chat 任务失败")

        job = workspace.get("job") if isinstance(workspace.get("job"), dict) else {}
        status = str(job.get("status") or "").strip()
        if status in OMNIDRIVE_OPENAI_PROXY_FINAL_JOB_STATUSES:
            break
        if time.time() - started_at >= 120:
            break
        time.sleep(2.5)

    job = workspace.get("job") if isinstance(workspace.get("job"), dict) else {}
    final_status = str(job.get("status") or "").strip()
    if final_status not in {"success", "completed"}:
        message = str(job.get("message") or "").strip() or "OmniDrive chat 任务未成功完成"
        raise RuntimeError(message)

    text = _extract_workspace_text(workspace)
    if not text:
        raise RuntimeError("OmniDrive chat 返回结果为空")

    effective_model_name = str(job.get("modelName") or model_name).strip() or model_name
    return {
        "jobId": job_id,
        "modelName": effective_model_name,
        "text": text,
    }


def ensure_publish_task_manager_started():
    if not publish_task_manager._started:
        task_logger.debug("ensuring publish task manager started")
    publish_task_manager.start()


def ensure_omnidrive_ai_task_manager_started():
    if not omnidrive_ai_task_manager._started:
        ai_logger.debug("ensuring omnidrive ai task manager started")
    omnidrive_ai_task_manager.start()


def ensure_cloud_agent_started():
    global cloud_agent

    if not CLOUD_AGENT_ENABLED or not CLOUD_DEMO_URL or not CLOUD_AGENT_KEY:
        return

    with cloud_agent_lock:
        if cloud_agent is None:
            agent_logger.info(
                "starting cloud agent device_code={} cloud_url={} poll_interval={} heartbeat_interval={}",
                DEVICE_CODE,
                CLOUD_DEMO_URL,
                CLOUD_AGENT_POLL_INTERVAL,
                CLOUD_AGENT_HEARTBEAT_INTERVAL,
            )
            cloud_agent = CloudAgent(
                cloud_base_url=CLOUD_DEMO_URL,
                agent_key=CLOUD_AGENT_KEY,
                run_login_fn=run_async_function,
                relay_fn=relay_remote_login_status,
                device_name=RESOLVED_DEVICE_NAME,
                poll_interval=CLOUD_AGENT_POLL_INTERVAL,
                heartbeat_interval=CLOUD_AGENT_HEARTBEAT_INTERVAL,
                device_code=DEVICE_CODE,
            )
            cloud_agent.start()


def ensure_omnidrive_agent_started():
    global omnidrive_agent

    if not OMNIDRIVE_AGENT_ENABLED or not OMNIDRIVE_BASE_URL or not OMNIDRIVE_AGENT_KEY:
        return

    with omnidrive_agent_lock:
        if omnidrive_agent is None:
            agent_logger.info(
                "starting omnidrive bridge device_code={} cloud_url={} poll_interval={} ai_poll_interval={} heartbeat_interval={}",
                DEVICE_CODE,
                OMNIDRIVE_BASE_URL,
                OMNIDRIVE_AGENT_POLL_INTERVAL,
                OMNIDRIVE_AGENT_AI_POLL_INTERVAL,
                OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL,
            )
            omnidrive_agent = OmniDriveBridge(
                db_path=Path(BASE_DIR / "db" / "database.db"),
                cloud_base_url=OMNIDRIVE_BASE_URL,
                agent_key=OMNIDRIVE_AGENT_KEY,
                run_login_fn=run_async_function,
                publish_task_manager=publish_task_manager,
                ai_task_manager=omnidrive_ai_task_manager,
                material_roots=OMNIBULL_MATERIAL_ROOTS,
                device_name=RESOLVED_DEVICE_NAME,
                device_code=DEVICE_CODE,
                device_fingerprint=DEVICE_IDENTITY.get("deviceFingerprint"),
                generated_root_name=OMNIBULL_GENERATED_ROOT_NAME,
                generated_root_path=OMNIBULL_GENERATED_ROOT_PATH,
                poll_interval=OMNIDRIVE_AGENT_POLL_INTERVAL,
                ai_poll_interval=OMNIDRIVE_AGENT_AI_POLL_INTERVAL,
                heartbeat_interval=OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL,
                account_sync_interval=OMNIDRIVE_ACCOUNT_SYNC_INTERVAL,
                material_sync_interval=OMNIDRIVE_MATERIAL_SYNC_INTERVAL,
                skill_sync_interval=OMNIDRIVE_SKILL_SYNC_INTERVAL,
                publish_sync_interval=OMNIDRIVE_PUBLISH_SYNC_INTERVAL,
                max_material_files=OMNIDRIVE_MATERIAL_SYNC_MAX_FILES,
            )
            omnidrive_agent.start()


def should_boot_background_services():
    flask_run_from_cli = os.environ.get('FLASK_RUN_FROM_CLI') == 'true'
    werkzeug_run_main = os.environ.get('WERKZEUG_RUN_MAIN') == 'true'

    if flask_run_from_cli:
        debug_enabled = parse_bool(os.environ.get('FLASK_DEBUG', False))
        if debug_enabled:
            return werkzeug_run_main
        return True
    return True


@app.before_request
def start_request_log_context():
    request.environ['_sau_started_at'] = time.perf_counter()
    request.environ['_sau_request_id'] = uuid.uuid4().hex[:12]


@app.after_request
def log_request_summary(response):
    started_at = request.environ.get('_sau_started_at')
    duration_ms = 0
    if started_at is not None:
        duration_ms = int((time.perf_counter() - started_at) * 1000)

    path = request.path
    request_id = request.environ.get('_sau_request_id')
    request_summary = _request_payload_summary()
    response_summary = None
    if response.status_code >= 400:
        try:
            response_summary = _compact_log_value(response.get_data(as_text=True))
        except Exception:
            response_summary = None

    log_message = (
        "http request completed request_id={} method={} path={} status={} duration_ms={} remote_addr={} payload={} response={}"
    )
    interval = _request_log_interval(path)
    level = "ERROR" if response.status_code >= 500 else "WARNING" if response.status_code >= 400 else "DEBUG"
    if interval:
        log_throttled(
            request_logger,
            level,
            f"http:{request.method}:{path}:{response.status_code}",
            interval,
            log_message,
            request_id,
            request.method,
            path,
            response.status_code,
            duration_ms,
            request.remote_addr,
            request_summary,
            response_summary,
        )
    else:
        request_logger.log(
            level,
            log_message,
            request_id,
            request.method,
            path,
            response.status_code,
            duration_ms,
            request.remote_addr,
            request_summary,
            response_summary,
        )
    return response


@app.before_request
def bootstrap_cloud_agent():
    ensure_publish_task_manager_started()
    ensure_omnidrive_ai_task_manager_started()
    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()

# 处理所有静态资源请求（未来打包用）
@app.route('/assets/<filename>')
def custom_static(filename):
    return send_from_directory(os.path.join(current_dir, 'assets'), filename)

# 处理 favicon.ico 静态资源（未来打包用）
@app.route('/favicon.ico')
def favicon():
    return send_from_directory(os.path.join(current_dir, 'assets'), 'vite.svg')

@app.route('/vite.svg')
def vite_svg():
    return send_from_directory(os.path.join(current_dir, 'assets'), 'vite.svg')

# （未来打包用）
@app.route('/')
def index():  # put application's code here
    return send_from_directory(current_dir, 'index.html')

@app.route('/upload', methods=['POST'])
def upload_file():
    """Store a raw upload under videoFile for immediate local publish use."""
    if 'file' not in request.files:
        return jsonify({
            "code": 400,
            "data": None,
            "msg": "No file part in the request"
        }), 400
    file = request.files['file']
    if file.filename == '':
        return jsonify({
            "code": 400,
            "data": None,
            "msg": "No selected file"
        }), 400
    try:
        # 保存文件到指定位置
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")
        filepath = Path(BASE_DIR / "videoFile" / f"{uuid_v1}_{file.filename}")
        file.save(filepath)
        return jsonify({"code":200,"msg": "File uploaded successfully", "data": f"{uuid_v1}_{file.filename}"}), 200
    except Exception as e:
        return jsonify({"code":500,"msg": str(e),"data":None}), 500

@app.route('/getFile', methods=['GET'])
def get_file():
    """Serve a previously uploaded local media file by its stored filename."""
    # 获取 filename 参数
    filename = request.args.get('filename')

    if not filename:
        return jsonify({"code": 400, "msg": "filename is required", "data": None}), 400

    # 防止路径穿越攻击
    if '..' in filename or filename.startswith('/'):
        return jsonify({"code": 400, "msg": "Invalid filename", "data": None}), 400

    # 拼接完整路径
    file_path = str(Path(BASE_DIR / "videoFile"))

    # 返回文件
    return send_from_directory(file_path,filename)


@app.route('/uploadSave', methods=['POST'])
def upload_save():
    """Persist a material-library asset and record it in the local SQLite catalog."""
    if 'file' not in request.files:
        return jsonify({
            "code": 400,
            "data": None,
            "msg": "No file part in the request"
        }), 400

    file = request.files['file']
    if file.filename == '':
        return jsonify({
            "code": 400,
            "data": None,
            "msg": "No selected file"
        }), 400

    # 获取表单中的自定义文件名（可选）
    custom_filename = request.form.get('filename', None)
    if custom_filename:
        filename = custom_filename + "." + file.filename.split('.')[-1]
    else:
        filename = file.filename

    try:
        # 生成 UUID v1
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")

        # 构造文件名和路径
        final_filename = f"{uuid_v1}_{filename}"
        filepath = Path(BASE_DIR / "videoFile" / f"{uuid_v1}_{filename}")

        # 保存文件
        file.save(filepath)

        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            cursor = conn.cursor()
            cursor.execute('''
                                INSERT INTO file_records (filename, filesize, file_path)
            VALUES (?, ?, ?)
                                ''', (filename, round(float(os.path.getsize(filepath)) / (1024 * 1024),2), final_filename))
            conn.commit()
            print("✅ 上传文件已记录")

        return jsonify({
            "code": 200,
            "msg": "File uploaded and saved successfully",
            "data": {
                "filename": filename,
                "filepath": final_filename
            }
        }), 200

    except Exception as e:
        print(f"Upload failed: {e}")
        return jsonify({
            "code": 500,
            "msg": f"upload failed: {e}",
            "data": None
        }), 500

@app.route('/getFiles', methods=['GET'])
def get_all_files():
    """Return all saved material records with a parsed UUID for frontend display."""
    try:
        # 使用 with 自动管理数据库连接
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row  # 允许通过列名访问结果
            cursor = conn.cursor()

            # 查询所有记录
            cursor.execute("SELECT * FROM file_records")
            rows = cursor.fetchall()

            # 将结果转为字典列表，并提取UUID
            data = []
            for row in rows:
                row_dict = dict(row)
                # 从 file_path 中提取 UUID (文件名的第一部分，下划线前)
                if row_dict.get('file_path'):
                    file_path_parts = row_dict['file_path'].split('_', 1)  # 只分割第一个下划线
                    if len(file_path_parts) > 0:
                        row_dict['uuid'] = file_path_parts[0]  # UUID 部分
                    else:
                        row_dict['uuid'] = ''
                else:
                    row_dict['uuid'] = ''
                data.append(row_dict)

            return jsonify({
                "code": 200,
                "msg": "success",
                "data": data
            }), 200
    except Exception as e:
        return jsonify({
            "code": 500,
            "msg": str("get file failed!"),
            "data": None
        }), 500


@app.route("/getAccounts", methods=['GET'])
def getAccounts():
    """快速获取所有账号信息，不进行cookie验证"""
    try:
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute('''
            SELECT * FROM user_info
            ORDER BY id DESC''')
            rows = cursor.fetchall()
            rows_list = [serialize_account_row(row) for row in rows]

            print("\n📋 当前数据表内容（快速获取）：")
            for row in rows:
                print(row)

            return jsonify(
                {
                    "code": 200,
                    "msg": None,
                    "data": rows_list
                }), 200
    except Exception as e:
        print(f"获取账号列表时出错: {str(e)}")
        return jsonify({
            "code": 500,
            "msg": f"获取账号列表失败: {str(e)}",
            "data": None
        }), 500


@app.route("/api/platforms", methods=["GET"])
def get_platforms():
    """Expose publish/login capability switches after syncing them from OmniDrive."""
    try:
        platform_items = refresh_platform_capabilities_from_omnidrive()
        return jsonify({
            "code": 200,
            "msg": "success",
            "data": platform_items,
        }), 200
    except Exception as exc:
        return jsonify({
            "code": 500,
            "msg": f"获取平台能力失败: {exc}",
            "data": None,
        }), 500


@app.route("/getValidAccounts",methods=['GET'])
async def getValidAccounts():
    """Validate every stored account cookie and return the refreshed row payloads."""
    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        cursor.execute('''
        SELECT * FROM user_info
        ORDER BY id DESC''')
        rows = cursor.fetchall()
        print("\n📋 当前数据表内容：")
        for row in rows:
            print(row)
        rows_list = await validate_account_rows(conn, rows)
        return jsonify(
                        {
                            "code": 200,
                            "msg": None,
                            "data": rows_list
                        }),200


@app.route("/validateAccount", methods=['GET'])
async def validateAccount():
    """Re-check a single account's login state so the frontend can surface cookie health."""
    account_id = request.args.get('id')

    if not account_id or not account_id.isdigit():
        return jsonify({
            "code": 400,
            "msg": "Invalid or missing account ID",
            "data": None
        }), 400

    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM user_info
            WHERE id = ?
            ''',
            (int(account_id),)
        )
        row = cursor.fetchone()

        if not row:
            return jsonify({
                "code": 404,
                "msg": "account not found",
                "data": None
            }), 404

        validated_rows = await validate_account_rows(conn, [row])

        return jsonify({
            "code": 200,
            "msg": "account validated successfully",
            "data": validated_rows[0]
        }), 200

@app.route('/deleteFile', methods=['GET'])
def delete_file():
    """Delete a material file from disk and remove its catalog row in one operation."""
    file_id = request.args.get('id')

    if not file_id or not file_id.isdigit():
        return jsonify({
            "code": 400,
            "msg": "Invalid or missing file ID",
            "data": None
        }), 400

    try:
        # 获取数据库连接
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            # 查询要删除的记录
            cursor.execute("SELECT * FROM file_records WHERE id = ?", (file_id,))
            record = cursor.fetchone()

            if not record:
                return jsonify({
                    "code": 404,
                    "msg": "File not found",
                    "data": None
                }), 404

            record = dict(record)

            # 获取文件路径并删除实际文件
            file_path = Path(BASE_DIR / "videoFile" / record['file_path'])
            if file_path.exists():
                try:
                    file_path.unlink()  # 删除文件
                    print(f"✅ 实际文件已删除: {file_path}")
                except Exception as e:
                    print(f"⚠️ 删除实际文件失败: {e}")
                    # 即使删除文件失败，也要继续删除数据库记录，避免数据不一致
            else:
                print(f"⚠️ 实际文件不存在: {file_path}")

            # 删除数据库记录
            cursor.execute("DELETE FROM file_records WHERE id = ?", (file_id,))
            conn.commit()

        return jsonify({
            "code": 200,
            "msg": "File deleted successfully",
            "data": {
                "id": record['id'],
                "filename": record['filename']
            }
        }), 200

    except Exception as e:
        return jsonify({
            "code": 500,
            "msg": str("delete failed!"),
            "data": None
        }), 500

@app.route('/deleteAccount', methods=['GET'])
def delete_account():
    """Delete a local account, clear cached storage state, and notify OmniDrive if available."""
    account_id = request.args.get('id')

    if not account_id or not account_id.isdigit():
        return jsonify({
            "code": 400,
            "msg": "Invalid or missing account ID",
            "data": None
        }), 400

    account_id = int(account_id)

    try:
        # 获取数据库连接
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            # 查询要删除的记录
            cursor.execute("SELECT * FROM user_info WHERE id = ?", (account_id,))
            record = cursor.fetchone()

            if not record:
                return jsonify({
                    "code": 404,
                    "msg": "account not found",
                    "data": None
                }), 404

            record = dict(record)
            clear_account_storage_state(record)

            # 删除数据库记录
            cursor.execute("DELETE FROM user_info WHERE id = ?", (account_id,))
            conn.commit()

            # 通知云端此账号已被删除
            global omnidrive_agent
            if omnidrive_agent:
                try:
                    omnidrive_agent._request("POST", "/api/v1/agent/accounts/sync", payload={
                        "deviceCode": omnidrive_agent.device_code,
                        "platform": PLATFORM_LABELS.get(int(record["type"] or 0), ""),
                        "accountName": record["userName"],
                        "status": "deleted",
                        "lastMessage": "Account explicitly deleted by user locally"
                    })
                except Exception as sync_exc:
                    print(f"⚠️ 云端销毁通知失败 (不影响本地删除): {sync_exc}")

        return jsonify({
            "code": 200,
            "msg": "account deleted successfully",
            "data": None
        }), 200

    except Exception as e:
        return jsonify({
            "code": 500,
            "msg": f"delete failed: {str(e)}",
            "data": None
        }), 500


@app.route('/api/accounts/<int:account_id>/open-backend', methods=['POST'])
def open_account_backend(account_id):
    return handle_open_backend_request(account_id, require_skill_auth=False)


# SSE 登录接口
@app.route('/login')
def login():
    """Start the platform-specific login worker and stream progress back over SSE."""
    type = request.args.get('type')
    id = str(request.args.get('id') or '').strip()

    if not type:
        return jsonify({"code": 400, "msg": "缺少平台类型", "data": None}), 400
    if not id:
        return jsonify({"code": 400, "msg": "账号名称不能为空", "data": None}), 400

    try:
        capability = ensure_platform_operation_enabled(type, "login")
    except ValueError as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400

    # 模拟一个用于异步通信的队列
    status_queue = Queue()
    active_queues[id] = status_queue

    def on_close():
        print(f"清理队列: {id}")
        active_queues.pop(id, None)
    # 启动异步任务线程
    thread = threading.Thread(
        target=run_async_function,
        args=(type, id, status_queue, None, False),
        daemon=True,
    )
    thread.start()
    response = Response(sse_stream(status_queue,), mimetype='text/event-stream')
    response.headers['Cache-Control'] = 'no-cache'
    response.headers['X-Accel-Buffering'] = 'no'  # 关键：禁用 Nginx 缓冲
    response.headers['Content-Type'] = 'text/event-stream'
    response.headers['Connection'] = 'keep-alive'
    response.headers['X-OmniBull-Platform'] = str(capability.get("type") or type)
    response.call_on_close(on_close)
    return response


@app.route('/cloudAgentStatus', methods=['GET'])
def cloud_agent_status():
    ensure_cloud_agent_started()
    agent_status = cloud_agent.status() if cloud_agent else None
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": {
            "config": get_cloud_agent_config(),
            "agent": agent_status
        }
    }), 200


@app.route('/omnidriveAgentStatus', methods=['GET'])
def omnidrive_agent_status():
    ensure_omnidrive_ai_task_manager_started()
    ensure_omnidrive_agent_started()
    agent_status = omnidrive_agent.status() if omnidrive_agent else None
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": {
            "config": get_omnidrive_agent_config(),
            "agent": agent_status,
        }
    }), 200

@app.route('/api/agent/forceSync', methods=['POST'])
def force_sync_cloud():
    ensure_omnidrive_agent_started()
    if omnidrive_agent:
        try:
            omnidrive_agent._sync_accounts()
            return jsonify({"code": 200, "msg": "Sync triggered successfully"}), 200
        except Exception as e:
            return jsonify({"code": 500, "msg": str(e)}), 500
    return jsonify({"code": 400, "msg": "Agent not running"}), 400


def create_local_ai_task(data, source="local_ui"):
    ensure_omnidrive_ai_task_manager_started()
    task = omnidrive_ai_task_manager.create_task(data, source=source)
    return task


def _shared_runtime_defaults():
    runtime_config = _load_shared_agent_runtime_config(refresh=False) or {}
    defaults = runtime_config.get("defaults") if isinstance(runtime_config.get("defaults"), dict) else {}
    return {
        "chatModel": str(defaults.get("chatModel") or "default-chat").strip() or "default-chat",
        "imageModel": str(defaults.get("imageModel") or "").strip() or None,
        "videoModel": str(defaults.get("videoModel") or "").strip() or None,
    }


def _build_openai_messages(prompt="", messages=None, system_prompt=None):
    if isinstance(messages, list) and messages:
        normalized_messages = []
        for item in messages:
            if not isinstance(item, dict):
                continue
            role = str(item.get("role") or "").strip()
            if not role:
                continue
            normalized_messages.append(
                {
                    "role": role,
                    "content": item.get("content"),
                }
            )
        if system_prompt and not any(str(item.get("role") or "").strip() == "system" for item in normalized_messages):
            normalized_messages.insert(0, {"role": "system", "content": str(system_prompt)})
        return normalized_messages

    normalized = []
    if system_prompt:
        normalized.append({"role": "system", "content": str(system_prompt)})
    normalized.append({"role": "user", "content": str(prompt or "").strip()})
    return normalized


def _public_source_to_internal_ai_source(source_category):
    if source_category == SOURCE_CATEGORY_OPENCLAW_VIA_HERMES:
        return "openclaw_skill"
    return "local_ui"


def _public_source_to_internal_publish_source(source_category):
    if source_category == SOURCE_CATEGORY_OPENCLAW_VIA_HERMES:
        return "openclaw_skill"
    return "local_api"


def _run_omnidrive_direct_chat(data, source_category, correlation_id):
    defaults = _shared_runtime_defaults()
    messages = _build_openai_messages(
        prompt=data.get("prompt") or data.get("input") or "",
        messages=data.get("messages"),
        system_prompt=data.get("systemPrompt") or data.get("instructions"),
    )
    completion = create_omnidrive_openai_chat_completion(
        {
            "model": str(data.get("model") or data.get("modelName") or defaults["chatModel"]).strip() or defaults["chatModel"],
            "messages": messages,
            "tools": data.get("tools") or [],
            "tool_choice": data.get("toolChoice"),
        }
    )
    return {
        "source": source_category,
        "executionEngine": "openclaw_direct",
        "correlationId": correlation_id,
        "fallback": False,
        "text": completion["text"],
        "jobId": completion["jobId"],
        "modelName": completion["modelName"],
    }


def _run_hermes_chat_request(data, source_category, correlation_id):
    prompt = str(data.get("prompt") or data.get("input") or "").strip()
    messages = data.get("messages") if isinstance(data.get("messages"), list) else None
    system_prompt = str(data.get("systemPrompt") or data.get("instructions") or "").strip() or None
    model_name = str(data.get("model") or data.get("modelName") or HERMES_PROFILE_NAME).strip() or HERMES_PROFILE_NAME
    if not prompt and not messages:
        raise ValueError("缺少 prompt 或 messages")

    if messages:
        request_payload = {
            "model": model_name,
            "messages": _build_openai_messages(messages=messages, system_prompt=system_prompt),
            "stream": False,
        }
        status_code, response_payload = hermes_api_json_request("POST", "/v1/chat/completions", payload=request_payload)
        if status_code >= 400:
            message = str(response_payload.get("error") or response_payload.get("message") or response_payload.get("detail") or "").strip()
            raise RuntimeError(message or "调用 Hermes chat/completions 失败")
        text = _extract_hermes_chat_completion_text(response_payload)
        if not text:
            raise RuntimeError("Hermes chat/completions 返回结果为空")
        return {
            "source": source_category,
            "executionEngine": "hermes",
            "correlationId": correlation_id,
            "fallback": False,
            "text": text,
            "jobId": None,
            "modelName": str(response_payload.get("model") or model_name).strip() or model_name,
            "response": response_payload,
        }

    request_payload = {
        "model": model_name,
        "input": prompt,
        "store": parse_bool(data.get("store", True)),
    }
    if system_prompt:
        request_payload["instructions"] = system_prompt
    conversation = str(data.get("conversation") or "").strip()
    previous_response_id = str(data.get("previousResponseId") or data.get("previous_response_id") or "").strip()
    if conversation:
        request_payload["conversation"] = conversation
    if previous_response_id:
        request_payload["previous_response_id"] = previous_response_id
    status_code, response_payload = hermes_api_json_request("POST", "/v1/responses", payload=request_payload)
    if status_code >= 400:
        message = str(response_payload.get("error") or response_payload.get("message") or response_payload.get("detail") or "").strip()
        raise RuntimeError(message or "调用 Hermes responses 失败")
    text = _extract_hermes_response_text(response_payload)
    if not text:
        raise RuntimeError("Hermes responses 返回结果为空")
    return {
        "source": source_category,
        "executionEngine": "hermes",
        "correlationId": correlation_id,
        "fallback": False,
        "text": text,
        "jobId": str(response_payload.get("id") or "").strip() or None,
        "modelName": str(response_payload.get("model") or model_name).strip() or model_name,
        "response": response_payload,
    }


def _enqueue_publish_from_bridge(data, *, source_category, execution_engine, correlation_id):
    account_file_paths = resolve_account_file_paths(
        account_ids=data.get("accountIds") or [],
        account_file_paths=data.get("accountFilePaths") or [],
    )
    file_items = resolve_skill_file_items(data.get("files") or [])
    publish_payload = {
        "type": data.get("platformType"),
        "title": str(data.get("title") or "").strip(),
        "tags": data.get("tags") or [],
        "accountList": account_file_paths,
        "fileItems": file_items,
        "runAt": data.get("runAt"),
        "enableTimer": 1 if parse_bool(data.get("enableTimer")) else 0,
        "videosPerDay": data.get("videosPerDay") or 1,
        "startDays": data.get("startDays") or 0,
        "dailyTimes": data.get("dailyTimes") or [],
        "category": data.get("category"),
        "isDraft": parse_bool(data.get("isDraft")),
        "productLink": data.get("productLink") or "",
        "productTitle": data.get("productTitle") or "",
        "sourceCategory": source_category,
        "executionEngine": execution_engine,
        "correlationId": correlation_id,
    }
    thumbnail = data.get("thumbnail")
    if thumbnail:
        publish_payload["thumbnailItem"] = resolve_skill_file_items([thumbnail])[0]
    ensure_publish_task_manager_started()
    internal_source = _public_source_to_internal_publish_source(source_category)
    return publish_task_manager.enqueue_from_request(publish_payload, source=internal_source)


def _create_ai_task_from_bridge(data, *, source_category, execution_engine, correlation_id, default_job_type=None):
    defaults = _shared_runtime_defaults()
    ai_payload = dict(data or {})
    ai_payload["jobType"] = str(ai_payload.get("jobType") or default_job_type or "").strip()
    if not ai_payload["jobType"]:
        raise ValueError("缺少 jobType")
    if not str(ai_payload.get("prompt") or "").strip():
        raise ValueError("缺少 prompt")
    if not str(ai_payload.get("modelName") or "").strip():
        if ai_payload["jobType"] == "image":
            ai_payload["modelName"] = defaults["imageModel"]
        elif ai_payload["jobType"] == "video":
            ai_payload["modelName"] = defaults["videoModel"]
        else:
            ai_payload["modelName"] = defaults["chatModel"]
    ai_payload["sourceCategory"] = source_category
    ai_payload["executionEngine"] = execution_engine
    ai_payload["correlationId"] = correlation_id
    internal_source = _public_source_to_internal_ai_source(source_category)
    return create_local_ai_task(ai_payload, source=internal_source)


def _run_bridge_skill(data, *, source_category, execution_engine, correlation_id):
    skill_name = str(data.get("skillName") or data.get("skill") or "").strip().lower()
    action = str(data.get("action") or "list").strip().lower()
    if not skill_name:
        raise ValueError("缺少 skillName")

    if skill_name == "omnibull-accounts":
        if action == "list":
            rows = fetch_account_rows()
            if parse_bool(data.get("validateCookies")):
                with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
                    conn.row_factory = sqlite3.Row
                    validated_rows = asyncio.run(validate_account_rows(conn, rows))
                status_map = {row[0]: row[-1] for row in validated_rows}
                payload = [serialize_account_detail(row, status_map.get(row["id"])) for row in rows]
            else:
                payload = [serialize_account_detail(row) for row in rows]
            return {"items": payload}
        if action == "detail":
            account_id = int(data.get("accountId"))
            rows = fetch_account_rows(account_ids=[account_id])
            if not rows:
                raise ValueError("账号不存在")
            return {"item": serialize_account_detail(rows[0])}
        if action == "validate":
            account_ids = data.get("accountIds") or []
            rows = fetch_account_rows(account_ids=account_ids if account_ids else None)
            with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
                conn.row_factory = sqlite3.Row
                validated_rows = asyncio.run(validate_account_rows(conn, rows))
            status_map = {row[0]: row[-1] for row in validated_rows}
            payload = [serialize_account_detail(row, status_map.get(row["id"])) for row in rows]
            return {"items": payload}
        raise ValueError(f"不支持的 omnibull-accounts action: {action}")

    if skill_name == "omnibull-materials":
        if action == "roots":
            return {"items": list_material_roots(OMNIBULL_MATERIAL_ROOTS)}
        if action == "list":
            return {
                "item": list_material_directory(
                    OMNIBULL_MATERIAL_ROOTS,
                    root_name=data.get("root"),
                    relative_path=data.get("path", ""),
                    limit=data.get("limit", 200),
                )
            }
        if action == "read":
            return {
                "item": read_material_file(
                    OMNIBULL_MATERIAL_ROOTS,
                    root_name=data.get("root"),
                    relative_path=data.get("path", ""),
                    max_bytes=data.get("maxBytes", 65536),
                )
            }
        raise ValueError(f"不支持的 omnibull-materials action: {action}")

    if skill_name == "omnibull-publish":
        if action == "enqueue":
            tasks = _enqueue_publish_from_bridge(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
            return {"taskCount": len(tasks), "tasks": tasks}
        if action == "tasks":
            ensure_publish_task_manager_started()
            return {
                "items": publish_task_manager.list_tasks(
                    limit=data.get("limit", 100),
                    status=data.get("status"),
                )
            }
        if action == "task_detail":
            ensure_publish_task_manager_started()
            task = publish_task_manager.get_task(str(data.get("taskUuid") or "").strip())
            if not task:
                raise ValueError("任务不存在")
            return {"item": task}
        raise ValueError(f"不支持的 omnibull-publish action: {action}")

    if skill_name == "omnidrive-chat":
        return _run_omnidrive_direct_chat(data, source_category, correlation_id)

    if skill_name == "omnidrive-image":
        task = _create_ai_task_from_bridge(
            data,
            source_category=source_category,
            execution_engine=execution_engine,
            correlation_id=correlation_id,
            default_job_type="image",
        )
        return {"item": task}

    if skill_name == "omnidrive-video":
        task = _create_ai_task_from_bridge(
            data,
            source_category=source_category,
            execution_engine=execution_engine,
            correlation_id=correlation_id,
            default_job_type="video",
        )
        return {"item": task}

    if skill_name == "omnidrive-jobs":
        ensure_omnidrive_ai_task_manager_started()
        if action == "detail":
            task = omnidrive_ai_task_manager.get_task(str(data.get("taskUuid") or "").strip())
            if not task:
                raise ValueError("AI 任务不存在")
            return {"item": task}
        return {
            "items": omnidrive_ai_task_manager.list_tasks(
                limit=data.get("limit", 100),
                status=data.get("status"),
                source=data.get("source"),
            )
        }

    raise ValueError(f"不支持的 Hermes skill: {skill_name}")


@app.route('/api/hermes/status', methods=['GET'])
def hermes_bridge_status():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_omnidrive_agent_started()
    ensure_openclaw_omnidrive_runtime_sync_started()

    runtime_refresh_error = None
    shared_runtime = _load_shared_agent_runtime_config(refresh=False)
    try:
        refreshed = refresh_openclaw_omnidrive_runtime_config(include_models=True)
        shared_runtime = refreshed.get("sharedRuntime") or shared_runtime
    except Exception as exc:
        runtime_refresh_error = str(exc)

    hermes_health = None
    hermes_models = None
    hermes_error = None
    try:
        health_status, hermes_health = hermes_api_json_request("GET", "/health", include_auth=False)
        models_status, hermes_models = hermes_api_json_request("GET", "/v1/models")
        hermes_ready = health_status < 400 and models_status < 400
    except Exception as exc:
        hermes_ready = False
        hermes_error = str(exc)

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": {
            "source": SOURCE_CATEGORY_HERMES_DIRECT,
            "executionEngine": "hermes",
            "correlationId": _ensure_correlation_id(request.args.get("correlationId")),
            "bridge": {
                "ready": hermes_ready and bool(shared_runtime),
                "fallbackChat": "omnidrive_openai_proxy",
                "runtimeRefreshError": runtime_refresh_error,
                "hermesError": hermes_error,
            },
            "hermesApi": {
                "baseUrl": HERMES_API_SERVER_BASE_URL,
                "profile": HERMES_PROFILE_NAME,
                "reachable": hermes_ready,
                "health": hermes_health,
                "models": hermes_models,
            },
            "sharedRuntime": _sanitize_shared_runtime_config_for_response(shared_runtime) if shared_runtime else None,
        },
    }), 200


@app.route('/api/hermes/chat', methods=['POST'])
def hermes_bridge_chat():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    data = request.get_json(silent=True) or {}
    source_category = _resolve_source_category(data.get("source"), SOURCE_CATEGORY_HERMES_DIRECT)
    correlation_id = _ensure_correlation_id(data.get("correlationId"))
    try:
        response_payload = _run_hermes_chat_request(data, source_category, correlation_id)
    except Exception as exc:
        try:
            response_payload = _run_omnidrive_direct_chat(data, source_category, correlation_id)
            response_payload["fallback"] = True
            response_payload["fallbackReason"] = str(exc)
        except Exception as fallback_exc:
            return jsonify({
                "code": 502,
                "msg": f"Hermes 不可用且直连回退失败: {fallback_exc}",
                "data": {
                    "source": source_category,
                    "executionEngine": "hermes",
                    "correlationId": correlation_id,
                    "fallback": False,
                    "error": str(exc),
                },
            }), 502

    return jsonify({"code": 200, "msg": "success", "data": response_payload}), 200


@app.route('/api/hermes/jobs/<job_id>', methods=['GET'])
def hermes_bridge_job_detail(job_id):
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    status_code, payload = hermes_api_json_request("GET", f"/v1/responses/{job_id}")
    if status_code >= 400:
        message = str(payload.get("error") or payload.get("message") or payload.get("detail") or "").strip()
        return jsonify({"code": status_code, "msg": message or "查询 Hermes job 失败", "data": payload}), status_code

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": {
            "source": SOURCE_CATEGORY_HERMES_DIRECT,
            "executionEngine": "hermes",
            "correlationId": _ensure_correlation_id(request.args.get("correlationId")),
            "jobId": str(payload.get("id") or job_id).strip() or job_id,
            "status": str(payload.get("status") or "").strip() or None,
            "text": _extract_hermes_response_text(payload),
            "response": payload,
        },
    }), 200


@app.route('/api/hermes/task/run', methods=['POST'])
def hermes_bridge_task_run():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    data = request.get_json(silent=True) or {}
    source_category = _resolve_source_category(data.get("source"), SOURCE_CATEGORY_HERMES_DIRECT)
    correlation_id = _ensure_correlation_id(data.get("correlationId"))
    execution_engine = "hermes"
    task_type = str(data.get("taskType") or data.get("action") or "run_skill").strip().lower()

    try:
        if task_type == "chat":
            result = _run_hermes_chat_request(data, source_category, correlation_id)
        elif task_type in {"skill", "run_skill"}:
            result = _run_bridge_skill(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
        elif task_type == "publish":
            tasks = _enqueue_publish_from_bridge(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
            result = {"taskCount": len(tasks), "tasks": tasks}
        elif task_type == "ai":
            task = _create_ai_task_from_bridge(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
            result = {"item": task}
        else:
            raise ValueError(f"不支持的 taskType: {task_type}")
    except Exception as exc:
        return jsonify({
            "code": 400,
            "msg": str(exc),
            "data": {
                "source": source_category,
                "executionEngine": execution_engine,
                "correlationId": correlation_id,
                "taskType": task_type,
            },
        }), 400

    if isinstance(result, dict):
        result.setdefault("source", source_category)
        result.setdefault("executionEngine", execution_engine)
        result.setdefault("correlationId", correlation_id)

    return jsonify({"code": 200, "msg": "success", "data": result}), 200


@app.route('/api/hermes/task/schedule', methods=['POST'])
def hermes_bridge_task_schedule():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    data = request.get_json(silent=True) or {}
    source_category = _resolve_source_category(data.get("source"), SOURCE_CATEGORY_HERMES_SCHEDULED)
    correlation_id = _ensure_correlation_id(data.get("correlationId"))
    execution_engine = "hermes"
    task_type = str(data.get("taskType") or data.get("action") or "publish").strip().lower()

    if not str(data.get("runAt") or "").strip() and not parse_bool(data.get("enableTimer")):
        return jsonify({
            "code": 400,
            "msg": "schedule 需要 runAt 或 enableTimer",
            "data": {
                "source": source_category,
                "executionEngine": execution_engine,
                "correlationId": correlation_id,
                "taskType": task_type,
            },
        }), 400

    try:
        if task_type in {"skill", "run_skill"}:
            result = _run_bridge_skill(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
        elif task_type == "publish":
            tasks = _enqueue_publish_from_bridge(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
            result = {"taskCount": len(tasks), "tasks": tasks}
        elif task_type == "ai":
            task = _create_ai_task_from_bridge(
                data,
                source_category=source_category,
                execution_engine=execution_engine,
                correlation_id=correlation_id,
            )
            result = {"item": task}
        else:
            raise ValueError(f"不支持的 taskType: {task_type}")
    except Exception as exc:
        return jsonify({
            "code": 400,
            "msg": str(exc),
            "data": {
                "source": source_category,
                "executionEngine": execution_engine,
                "correlationId": correlation_id,
                "taskType": task_type,
            },
        }), 400

    if isinstance(result, dict):
        result.setdefault("source", source_category)
        result.setdefault("executionEngine", execution_engine)
        result.setdefault("correlationId", correlation_id)

    return jsonify({"code": 200, "msg": "success", "data": result}), 200

@app.route('/aiTasks', methods=['GET', 'POST'])
def local_ai_tasks():
    if request.method == 'POST':
        try:
            payload = request.get_json(silent=True) or {}
            payload.setdefault("sourceCategory", "omnibull_local")
            payload.setdefault("executionEngine", "omnibull")
            payload.setdefault("correlationId", _ensure_correlation_id(payload.get("correlationId")))
            task = create_local_ai_task(payload, source="local_ui")
            return jsonify({"code": 200, "msg": "success", "data": task}), 200
        except ValueError as exc:
            return jsonify({"code": 400, "msg": str(exc), "data": None}), 400
        except Exception as exc:
            return jsonify({"code": 500, "msg": f"创建 AI 任务失败: {exc}", "data": None}), 500

    ensure_omnidrive_ai_task_manager_started()
    limit = request.args.get('limit', 100)
    status = str(request.args.get('status') or '').strip() or None
    source = str(request.args.get('source') or '').strip() or None
    try:
        tasks = omnidrive_ai_task_manager.list_tasks(limit=limit, status=status, source=source)
        return jsonify({"code": 200, "msg": "success", "data": tasks}), 200
    except Exception as exc:
        return jsonify({"code": 500, "msg": f"获取 AI 任务失败: {exc}", "data": None}), 500


@app.route('/aiTaskDetail', methods=['GET'])
def local_ai_task_detail():
    ensure_omnidrive_ai_task_manager_started()
    task_uuid = str(request.args.get('taskUuid') or request.args.get('id') or '').strip()
    if not task_uuid:
        return jsonify({"code": 400, "msg": "taskUuid 不能为空", "data": None}), 400
    task = omnidrive_ai_task_manager.get_task(task_uuid)
    if not task:
        return jsonify({"code": 404, "msg": "AI 任务不存在", "data": None}), 404
    return jsonify({"code": 200, "msg": "success", "data": task}), 200


@app.route('/api/skill/status', methods=['GET'])
def skill_status():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()
    ensure_openclaw_omnidrive_runtime_sync_started()
    payload = build_skill_status_payload()
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": payload,
    }), 200


@app.route('/api/skill/omnidrive/session', methods=['GET'])
def skill_omnidrive_session():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    try:
        status_code, payload = fetch_omnidrive_device_session()
    except Exception as exc:
        return jsonify({
            "code": 500,
            "msg": f"获取 OmniDrive 设备会话失败: {exc}",
            "data": None,
        }), 500

    if status_code >= 400:
        message = ""
        if isinstance(payload, dict):
            message = str(payload.get("error") or payload.get("message") or "").strip()
        summary = _build_omnidrive_authorization_summary(reason=message or "获取 OmniDrive 设备会话失败")
        return jsonify({
            "code": status_code,
            "msg": message or "获取 OmniDrive 设备会话失败",
            "data": summary,
        }), status_code

    try:
        device = payload.get("device") if isinstance(payload.get("device"), dict) else {}
        model_items = None
        if _should_refresh_openclaw_omnidrive_model_cache():
            try:
                model_status_code, model_payload = omnidrive_cloud_json_request(
                    "GET",
                    "/api/v1/ai/models",
                    access_token=payload.get("accessToken"),
                    query={"category": "chat"},
                    timeout=30,
                    api_base_url=payload.get("apiBaseUrl") or payload.get("cloudUrl"),
                )
                if model_status_code < 400:
                    model_items = _extract_omnidrive_chat_models(model_payload)
                    _mark_openclaw_omnidrive_model_sync()
                else:
                    message = ""
                    if isinstance(model_payload, dict):
                        message = str(model_payload.get("error") or model_payload.get("message") or "").strip()
                    app_logger.warning(
                        "skip OpenClaw OmniDrive model refresh after session fetch status={} message={}",
                        model_status_code,
                        message or "unknown error",
                    )
            except Exception as exc:
                app_logger.warning("refresh OpenClaw OmniDrive models after session fetch failed error={}", exc)

        sync_openclaw_omnidrive_model_configs(
            model_items,
            api_base_url=payload.get("apiBaseUrl") or payload.get("cloudUrl"),
            access_token=payload.get("accessToken"),
            default_chat_model=device.get("defaultChatModel"),
        )
        sync_hermes_shared_runtime_config(
            model_items,
            api_base_url=payload.get("apiBaseUrl") or payload.get("cloudUrl"),
            access_token=payload.get("accessToken"),
            device=device,
        )
    except Exception as exc:
        app_logger.warning("sync OpenClaw OmniDrive runtime connection after session fetch failed error={}", exc)

    response_payload = dict(payload) if isinstance(payload, dict) else {}
    response_payload.update(_build_omnidrive_authorization_summary(payload))

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": response_payload,
    }), 200


@app.route(f'{OMNIDRIVE_OPENAI_PROXY_BASE_PATH}/models', methods=['GET'])
def omnidrive_openai_models():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    try:
        session = get_omnidrive_device_session_data()
        status_code, payload = omnidrive_cloud_json_request(
            "GET",
            "/api/v1/ai/models",
            access_token=session["accessToken"],
            query={"category": "chat"},
            timeout=30,
            api_base_url=session.get("apiBaseUrl"),
        )
    except Exception as exc:
        return _openai_error_response(str(exc), status_code=500, error_type="server_error")

    if status_code >= 400:
        message = ""
        if isinstance(payload, dict):
            message = str(payload.get("error") or payload.get("message") or "").strip()
        return _openai_error_response(message or "列出 OmniDrive 模型失败", status_code=status_code, error_type="server_error")

    try:
        sync_openclaw_omnidrive_model_configs(
            _extract_omnidrive_chat_models(payload),
            api_base_url=session.get("apiBaseUrl"),
            access_token=session.get("accessToken"),
            default_chat_model=session.get("device", {}).get("defaultChatModel"),
        )
        _mark_openclaw_omnidrive_model_sync()
    except Exception as exc:
        app_logger.warning("sync OpenClaw OmniDrive model list after models request failed error={}", exc)

    model_items = []
    model_items.append(
        {
            "id": "default-chat",
            "object": "model",
            "created": 0,
            "owned_by": "omnidrive",
        }
    )
    for item in payload or []:
        if not isinstance(item, dict):
            continue
        model_id = str(item.get("modelName") or item.get("name") or item.get("id") or "").strip()
        if not model_id:
            continue
        model_items.append(
            {
                "id": model_id,
                "object": "model",
                "created": 0,
                "owned_by": "omnidrive",
            }
        )

    return jsonify({
        "object": "list",
        "data": model_items,
    }), 200


@app.route(f'{OMNIDRIVE_OPENAI_PROXY_BASE_PATH}/chat/completions', methods=['POST'])
def omnidrive_openai_chat_completions():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    payload = request.get_json(silent=True) or {}
    if parse_bool(payload.get("stream")):
        try:
            return stream_omnidrive_openai_chat_completion(payload)
        except ValueError as exc:
            return _openai_error_response(str(exc), status_code=400)
        except RuntimeError as exc:
            return _openai_error_response(str(exc), status_code=502, error_type="server_error")
        except Exception as exc:
            return _openai_error_response(str(exc), status_code=500, error_type="server_error")

    try:
        completion = create_omnidrive_openai_chat_completion(payload)
    except ValueError as exc:
        return _openai_error_response(str(exc), status_code=400)
    except RuntimeError as exc:
        return _openai_error_response(str(exc), status_code=502, error_type="server_error")
    except Exception as exc:
        return _openai_error_response(str(exc), status_code=500, error_type="server_error")

    if completion.get("toolCall"):
        return jsonify(
            _build_openai_tool_call_completion_response(
                completion["jobId"],
                completion["modelName"],
                completion["toolCall"],
            )
        ), 200

    return jsonify(
        _build_openai_chat_completion_response(
            completion["jobId"],
            completion["modelName"],
            completion["text"],
        )
    ), 200


@app.route('/api/skill/omnidrive/skills', methods=['GET'])
def skill_omnidrive_skills():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_omnidrive_agent_started()
    include_assets = str(request.args.get('includeAssets') or '').strip().lower() in {'1', 'true', 'yes'}
    skills = omnidrive_agent.list_cached_skills(include_assets=include_assets) if omnidrive_agent else []
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": skills,
    }), 200


@app.route('/api/skill/omnidrive/skills/<skill_id>', methods=['GET'])
def skill_omnidrive_skill_detail(skill_id):
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_omnidrive_agent_started()
    detail = omnidrive_agent.get_cached_skill(skill_id) if omnidrive_agent else None
    if not detail:
        return jsonify({
            "code": 404,
            "msg": "本地未找到已同步的 OmniDrive 技能包",
            "data": None,
        }), 404

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": detail,
    }), 200


@app.route('/api/skill/accounts', methods=['GET'])
async def skill_accounts():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    validate_cookies = parse_bool(request.args.get('validate'))
    rows = fetch_account_rows()

    if validate_cookies:
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            validated_rows = await validate_account_rows(conn, rows)
        status_map = {row[0]: row[-1] for row in validated_rows}
        data = [serialize_account_detail(row, status_map.get(row["id"])) for row in rows]
    else:
        data = [serialize_account_detail(row) for row in rows]

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": data,
    }), 200


@app.route('/api/skill/accounts/<int:account_id>', methods=['GET'])
def skill_account_detail(account_id):
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    rows = fetch_account_rows(account_ids=[account_id])
    if not rows:
        return jsonify({"code": 404, "msg": "账号不存在", "data": None}), 404

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": serialize_account_detail(rows[0]),
    }), 200


@app.route('/api/skill/accounts/<int:account_id>/open-backend', methods=['POST'])
def skill_account_open_backend(account_id):
    return handle_open_backend_request(account_id, require_skill_auth=True)


@app.route('/api/skill/accounts/validate', methods=['POST'])
async def skill_accounts_validate():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    data = request.get_json(silent=True) or {}
    validate_all = parse_bool(data.get("validateAll"))
    account_ids = data.get("accountIds") or []
    account_id = data.get("accountId")

    if account_id is not None:
        account_ids.append(account_id)

    if validate_all:
        rows = fetch_account_rows()
    else:
        normalized_ids = []
        for value in account_ids:
            try:
                normalized_ids.append(int(value))
            except (TypeError, ValueError):
                continue
        if not normalized_ids:
            return jsonify({"code": 400, "msg": "缺少可校验的账号ID", "data": None}), 400
        rows = fetch_account_rows(account_ids=normalized_ids)

    if not rows:
        return jsonify({"code": 404, "msg": "未找到可校验账号", "data": None}), 404

    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        validated_rows = await validate_account_rows(conn, rows)

    status_map = {row[0]: row[-1] for row in validated_rows}
    payload = [serialize_account_detail(row, status_map.get(row["id"])) for row in rows]
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": payload,
    }), 200


@app.route('/api/skill/materials/roots', methods=['GET'])
def skill_material_roots():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": list_material_roots(OMNIBULL_MATERIAL_ROOTS),
    }), 200


@app.route('/api/skill/materials/list', methods=['GET'])
def skill_material_list():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    root_name = request.args.get("root")
    relative_path = request.args.get("path", "")
    if not root_name:
        return jsonify({"code": 400, "msg": "缺少 root 参数", "data": None}), 400

    try:
        payload = list_material_directory(
            OMNIBULL_MATERIAL_ROOTS,
            root_name=root_name,
            relative_path=relative_path,
            limit=request.args.get("limit", 200),
        )
    except Exception as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": payload,
    }), 200


@app.route('/api/skill/materials/file', methods=['GET'])
def skill_material_file():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    root_name = request.args.get("root")
    relative_path = request.args.get("path", "")
    if not root_name or not relative_path:
        return jsonify({"code": 400, "msg": "缺少 root 或 path 参数", "data": None}), 400

    try:
        payload = read_material_file(
            OMNIBULL_MATERIAL_ROOTS,
            root_name=root_name,
            relative_path=relative_path,
            max_bytes=request.args.get("maxBytes", 65536),
        )
    except Exception as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": payload,
    }), 200


@app.route('/api/skill/ai/tasks', methods=['GET', 'POST'])
def skill_ai_tasks():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    if request.method == 'POST':
        data = request.get_json(silent=True) or {}
        try:
            data.setdefault("sourceCategory", SOURCE_CATEGORY_OPENCLAW_DIRECT)
            data.setdefault("executionEngine", "openclaw")
            data.setdefault("correlationId", _ensure_correlation_id(data.get("correlationId")))
            task = create_local_ai_task(data, source="openclaw_skill")
            return jsonify({"code": 200, "msg": "success", "data": task}), 200
        except ValueError as exc:
            return jsonify({"code": 400, "msg": str(exc), "data": None}), 400
        except Exception as exc:
            return jsonify({"code": 500, "msg": f"创建 AI 任务失败: {exc}", "data": None}), 500

    ensure_omnidrive_ai_task_manager_started()
    limit = request.args.get('limit', 100)
    status = str(request.args.get('status') or '').strip() or None
    source = str(request.args.get('source') or '').strip() or None
    tasks = omnidrive_ai_task_manager.list_tasks(limit=limit, status=status, source=source)
    return jsonify({"code": 200, "msg": "success", "data": tasks}), 200


@app.route('/api/skill/ai/tasks/<task_uuid>', methods=['GET'])
def skill_ai_task_detail(task_uuid):
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_omnidrive_ai_task_manager_started()
    task = omnidrive_ai_task_manager.get_task(task_uuid)
    if not task:
        return jsonify({"code": 404, "msg": "AI 任务不存在", "data": None}), 404
    return jsonify({"code": 200, "msg": "success", "data": task}), 200


@app.route('/api/skill/publish', methods=['POST'])
def skill_publish():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    data = request.get_json(silent=True) or {}
    platform_type = data.get("platformType")
    title = str(data.get("title") or "").strip()
    files = data.get("files") or []

    if platform_type is None:
        return jsonify({"code": 400, "msg": "缺少 platformType", "data": None}), 400
    if not title:
        return jsonify({"code": 400, "msg": "缺少 title", "data": None}), 400
    if not files:
        return jsonify({"code": 400, "msg": "缺少 files", "data": None}), 400

    try:
        account_file_paths = resolve_account_file_paths(
            account_ids=data.get("accountIds") or [],
            account_file_paths=data.get("accountFilePaths") or [],
        )
        file_items = resolve_skill_file_items(files)
        publish_payload = {
            "type": platform_type,
            "title": title,
            "tags": data.get("tags") or [],
            "accountList": account_file_paths,
            "fileItems": file_items,
            "runAt": data.get("runAt"),
            "enableTimer": 1 if parse_bool(data.get("enableTimer")) else 0,
            "videosPerDay": data.get("videosPerDay") or 1,
            "startDays": data.get("startDays") or 0,
            "dailyTimes": data.get("dailyTimes") or [],
            "category": data.get("category"),
            "isDraft": parse_bool(data.get("isDraft")),
            "productLink": data.get("productLink") or "",
            "productTitle": data.get("productTitle") or "",
            "sourceCategory": SOURCE_CATEGORY_OPENCLAW_DIRECT,
            "executionEngine": "openclaw",
            "correlationId": _ensure_correlation_id(data.get("correlationId")),
        }
        thumbnail = data.get("thumbnail")
        if thumbnail:
            publish_payload["thumbnailItem"] = resolve_skill_file_items([thumbnail])[0]

        ensure_publish_task_manager_started()
        tasks = publish_task_manager.enqueue_from_request(publish_payload, source="openclaw_skill")
    except Exception as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400

    return jsonify({
        "code": 200,
        "msg": "发布任务已入队",
        "data": {
            "taskCount": len(tasks),
            "tasks": tasks,
        },
    }), 200


@app.route('/api/skill/publish/tasks', methods=['GET'])
def skill_publish_tasks():
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_publish_task_manager_started()
    tasks = publish_task_manager.list_tasks(
        limit=request.args.get("limit", 100),
        status=request.args.get("status"),
    )
    return jsonify({
        "code": 200,
        "msg": "success",
        "data": tasks,
    }), 200


@app.route('/api/skill/publish/tasks/<task_uuid>', methods=['GET'])
def skill_publish_task_detail(task_uuid):
    auth_error = ensure_skill_api_authorized()
    if auth_error:
        return auth_error

    ensure_publish_task_manager_started()
    task = publish_task_manager.get_task(task_uuid)
    if not task:
        return jsonify({"code": 404, "msg": "任务不存在", "data": None}), 404

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": task,
    }), 200

@app.route('/postVideo', methods=['POST'])
def postVideo():
    """Validate a single publish request and enqueue it into the local publish manager."""
    data = request.get_json()

    if not data:
        return jsonify({"code": 400, "msg": "请求数据不能为空", "data": None}), 400

    file_list = data.get('fileList', [])
    account_list = data.get('accountList', [])
    type = data.get('type')
    title = data.get('title')

    if not file_list:
        return jsonify({"code": 400, "msg": "文件列表不能为空", "data": None}), 400
    if not account_list:
        return jsonify({"code": 400, "msg": "账号列表不能为空", "data": None}), 400
    if not type:
        return jsonify({"code": 400, "msg": "平台类型不能为空", "data": None}), 400
    if not title:
        return jsonify({"code": 400, "msg": "标题不能为空", "data": None}), 400

    print("File List:", file_list)
    print("Account List:", account_list)

    try:
        ensure_publish_task_manager_started()
        data.setdefault("sourceCategory", "omnibull_local")
        data.setdefault("executionEngine", "omnibull")
        data.setdefault("correlationId", _ensure_correlation_id(data.get("correlationId")))
        tasks = publish_task_manager.enqueue_from_request(data, source="local_api")
    except ValueError as exc:
        return jsonify({
            "code": 400,
            "msg": str(exc),
            "data": None,
        }), 400
    except Exception as e:
        print(f"发布视频时出错: {str(e)}")
        return jsonify({
            "code": 500,
            "msg": f"发布失败: {str(e)}",
            "data": None
        }), 500

    return jsonify(
        {
            "code": 200,
            "msg": "发布任务已入队",
            "data": {
                "taskCount": len(tasks),
                "tasks": tasks,
            }
        }), 200


@app.route('/updateUserinfo', methods=['POST'])
def updateUserinfo():
    # 获取JSON数据
    data = request.get_json()

    # 从JSON数据中提取 type 和 userName
    user_id = data.get('id')
    type = data.get('type')
    userName = data.get('userName')
    try:
        # 获取数据库连接
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()

            # 更新数据库记录
            cursor.execute('''
                           UPDATE user_info
                           SET type     = ?,
                               userName = ?
                           WHERE id = ?;
                           ''', (type, userName, user_id))
            conn.commit()

        return jsonify({
            "code": 200,
            "msg": "account update successfully",
            "data": None
        }), 200

    except Exception as e:
        return jsonify({
            "code": 500,
            "msg": str("update failed!"),
            "data": None
        }), 500

@app.route('/postVideoBatch', methods=['POST'])
def postVideoBatch():
    """Validate and enqueue a batch of publish requests from the multi-tab frontend."""
    data_list = request.get_json()

    if not isinstance(data_list, list):
        return jsonify({"code": 400, "msg": "Expected a JSON array", "data": None}), 400
    try:
        ensure_publish_task_manager_started()
        all_tasks = []
        for data in data_list:
            print("File List:", data.get('fileList', []))
            print("Account List:", data.get('accountList', []))
            data.setdefault("sourceCategory", "omnibull_local")
            data.setdefault("executionEngine", "omnibull")
            data.setdefault("correlationId", _ensure_correlation_id(data.get("correlationId")))
            all_tasks.extend(publish_task_manager.enqueue_from_request(data, source="local_batch_api"))
    except ValueError as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400
    except Exception as exc:
        return jsonify({"code": 500, "msg": f"批量发布入队失败: {exc}", "data": None}), 500

    return jsonify(
        {
            "code": 200,
            "msg": "批量发布任务已入队",
            "data": {
                "taskCount": len(all_tasks),
                "tasks": all_tasks,
            }
        }), 200


@app.route('/publishTasks', methods=['GET'])
def get_publish_tasks():
    """List local publish tasks so operators can inspect queue state and outcomes."""
    ensure_publish_task_manager_started()
    status = request.args.get('status')
    limit = request.args.get('limit', 100)

    try:
        tasks = publish_task_manager.list_tasks(limit=limit, status=status)
    except Exception as exc:
        return jsonify({"code": 500, "msg": f"获取发布任务失败: {exc}", "data": None}), 500

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": tasks,
    }), 200


@app.route('/publishTaskDetail', methods=['GET'])
def get_publish_task_detail():
    """Return one publish task by UUID for retry and diagnostics views."""
    ensure_publish_task_manager_started()
    task_uuid = request.args.get('id') or request.args.get('taskUuid')
    if not task_uuid:
        return jsonify({"code": 400, "msg": "缺少任务ID", "data": None}), 400

    task = publish_task_manager.get_task(task_uuid)
    if not task:
        return jsonify({"code": 404, "msg": "任务不存在", "data": None}), 404

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": task,
    }), 200

@app.route('/retryPublishTask', methods=['POST'])
def retry_publish_task():
    """Move an eligible failed/cancelled task back to pending for manual retry."""
    try:
        ensure_publish_task_manager_started()
        data = request.json or {}
        task_uuid = data.get("uuid")
        if not task_uuid:
            return jsonify({"code": 400, "msg": "缺少任务 UUID", "data": None}), 400
            
        task = publish_task_manager.get_task(task_uuid)
        if not task:
            return jsonify({"code": 404, "msg": "任务不存在", "data": None}), 404
            
        status = str(task.get("status") or "")
        if status not in {"failed", "needs_verify", "cancelled"}:
            return jsonify({"code": 400, "msg": "只有失败、取消或需要验证的任务才能重试", "data": None}), 400

        with publish_task_manager._connect() as conn:
            cursor = conn.cursor()
            cursor.execute(
                '''
                UPDATE publish_tasks
                SET status = 'pending',
                    message = '人工重试等待执行',
                    auto_retry_count = 0,
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                ''',
                (task_uuid,)
            )
            changed = cursor.rowcount == 1
            conn.commit()

        if changed:
            publish_task_manager._sync_task(task_uuid)
            return jsonify({"code": 200, "msg": "任务已重新加入队列"})
        return jsonify({"code": 500, "msg": "重试失败"}), 500
    except Exception as exc:
        return jsonify({"code": 500, "msg": f"重试任务失败: {exc}"}), 500

# Cookie文件上传API
@app.route('/uploadCookie', methods=['POST'])
def upload_cookie():
    """Import a browser cookie snapshot into the selected local account record."""
    try:
        if 'file' not in request.files:
            return jsonify({
                "code": 400,
                "msg": "没有找到Cookie文件",
                "data": None
            }), 400

        file = request.files['file']
        if file.filename == '':
            return jsonify({
                "code": 400,
                "msg": "Cookie文件名不能为空",
                "data": None
            }), 400

        if not file.filename.endswith('.json'):
            return jsonify({
                "code": 400,
                "msg": "Cookie文件必须是JSON格式",
                "data": None
            }), 400

        # 获取账号信息
        account_id = request.form.get('id')

        if not account_id or not str(account_id).isdigit():
            return jsonify({
                "code": 400,
                "msg": "缺少或错误的账号ID",
                "data": None
            }), 400

        account_id = int(account_id)
        account_row = get_account_row(account_id)
        if not account_row:
            return jsonify({
                "code": 404,
                "msg": "账号不存在",
                "data": None
            }), 404

        import_account_storage_state(account_id, file.read())

        return jsonify({
            "code": 200,
            "msg": "Cookie文件上传成功",
            "data": serialize_account_detail(get_account_row(account_row["id"]))
        }), 200

    except Exception as e:
        print(f"上传Cookie文件时出错: {str(e)}")
        return jsonify({
            "code": 500,
            "msg": f"上传Cookie文件失败: {str(e)}",
            "data": None
        }), 500


# Cookie文件下载API
@app.route('/downloadCookie', methods=['GET'])
def download_cookie():
    """Export the stored cookie payload for a specific local account."""
    try:
        account_id = request.args.get('id')
        file_path = request.args.get('filePath')
        if not account_id and not file_path:
            return jsonify({
                "code": 400,
                "msg": "缺少账号ID或文件路径参数",
                "data": None
            }), 400

        account_ref = int(account_id) if account_id and str(account_id).isdigit() else file_path
        account_row = get_account_row(account_ref)
        if not account_row:
            return jsonify({
                "code": 404,
                "msg": "账号不存在",
                "data": None
            }), 404

        payload = export_account_storage_state(account_row["id"])
        download_name = str(account_row.get("filePath") or "cookie.json").strip() or "cookie.json"
        return Response(
            payload,
            mimetype='application/json',
            headers={
                'Content-Disposition': f'attachment; filename="{download_name}"'
            },
        )

    except Exception as e:
        print(f"下载Cookie文件时出错: {str(e)}")
        return jsonify({
            "code": 500,
            "msg": f"下载Cookie文件失败: {str(e)}",
            "data": None
        }), 500


# 包装函数：在线程中运行异步函数
def run_async_function(type, id, status_queue, command_queue=None, keep_browser_open_on_success=False):
    """Run the platform login coroutine inside a worker thread and mirror status to SSE queues."""
    try:
        # First, attempt local fast-path validation if an existing cookie is present
        try:
            with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
                conn.row_factory = sqlite3.Row
                cursor = conn.cursor()
                cursor.execute("SELECT * FROM user_info WHERE userName = ? AND type = ?", (id, type))
                row = cursor.fetchone()
                
                if row:
                    loop = asyncio.new_event_loop()
                    asyncio.set_event_loop(loop)
                    result = loop.run_until_complete(check_cookie_detail(int(type), row['filePath'], headless=True))
                    loop.close()
                    is_valid = bool(result.get("ok"))
                    failure_message = str(result.get("message") or "").strip() or "本地 cookie 当前不可用"
                    update_account_runtime_status(
                        row["id"],
                        1 if is_valid else 0,
                        None if is_valid else failure_message,
                    )
                    if is_valid:
                        print(f"{id} 本地 Cookie 验证成功，直接进入等效登录完成状态。")
                        if status_queue is not None:
                            status_queue.put("200")
                        return
                    else:
                        print(f"{id} 本地 Cookie 验证失效或需二次认证，进入扫码登录流程...")
                        if status_queue is not None and failure_message:
                            status_queue.put(failure_message)
        except Exception as precheck_exc:
            print(f"预先检查本地 Cookie 失效: {precheck_exc}")

        match type:
            case '1':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(
                    xiaohongshu_cookie_gen(
                        id,
                        status_queue,
                        command_queue,
                        keep_browser_open_on_success,
                    )
                )
                loop.close()
            case '2':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(
                    get_tencent_cookie(
                        id,
                        status_queue,
                        command_queue,
                        keep_browser_open_on_success,
                    )
                )
                loop.close()
            case '3':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(
                    douyin_cookie_gen(
                        id,
                        status_queue,
                        command_queue,
                        keep_browser_open_on_success,
                    )
                )
                loop.close()
            case '4':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(
                    get_ks_cookie(
                        id,
                        status_queue,
                        command_queue,
                        keep_browser_open_on_success,
                    )
                )
                loop.close()
            case _:
                raise ValueError(f"unsupported login type: {type}")
    except Exception as exc:
        login_logger.exception("login thread execution failed platform_type={} account_name={} error={}", type, id, exc)
        if status_queue is not None:
            push_login_failed_status(status_queue, command_queue, str(exc))


if should_boot_background_services():
    ensure_publish_task_manager_started()
    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()
    ensure_openclaw_omnidrive_models_synced()
    ensure_openclaw_omnidrive_runtime_sync_started()
    ensure_openclaw_gateway_daily_reload_started()
# SSE 流生成器函数
def sse_stream(status_queue):
    """Convert queued login status messages into server-sent-event frames for the frontend."""
    yield f": {' ' * 2048}\n\n"
    while True:
        if not status_queue.empty():
            msg = status_queue.get()
            if isinstance(msg, dict):
                event_type = msg.get("type", "message")
                payload = msg.get("payload") or {}
                data_str = json.dumps(msg, ensure_ascii=False)
                if event_type == "qr_status":
                    yield f"event: qr\ndata: {data_str}\n\n"
                elif event_type in {"error", "login_failed"}:
                    message = str(payload.get("message") or "").strip() or "登录失败，请重试"
                    yield f"event: login_failed\ndata: {message}\n\n"
                    yield "data: " + json.dumps({
                        "type": "login_failed",
                        "payload": {"message": message},
                    }, ensure_ascii=False) + "\n\n"
                    break
                else:
                    yield f"event: {event_type}\ndata: {data_str}\n\n"
            elif isinstance(msg, str):
                if msg.startswith("data:image/"):
                    yield f"event: qr\ndata: \n\n"
                elif msg == "200":
                    yield f"event: done\ndata: 200\n\n"
                    yield "data: " + json.dumps({
                        "type": "done",
                        "payload": {"message": "登录成功"},
                    }, ensure_ascii=False) + "\n\n"
                    break
                elif msg == "CANCELLED":
                    yield f"event: cancelled\ndata: 本地登录浏览器已关闭，本次添加账号未完成\n\n"
                    yield "data: " + json.dumps({
                        "type": "cancelled",
                        "payload": {"message": "本地登录浏览器已关闭，本次添加账号未完成"},
                    }, ensure_ascii=False) + "\n\n"
                    break
                elif msg == "500":
                    yield f"event: login_failed\ndata: 内部错误\n\n"
                    yield "data: " + json.dumps({
                        "type": "login_failed",
                        "payload": {"message": "内部错误"},
                    }, ensure_ascii=False) + "\n\n"
                    break
                else:
                    yield f"data: {msg}\n\n"
        else:
            # 避免 CPU 占满
            time.sleep(0.1)

if __name__ == '__main__':
    ensure_publish_task_manager_started()
    ensure_omnidrive_ai_task_manager_started()
    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()
    ensure_openclaw_omnidrive_models_synced()
    ensure_openclaw_omnidrive_runtime_sync_started()
    ensure_openclaw_gateway_daily_reload_started()
    app.run(host='0.0.0.0', port=SAU_BACKEND_PORT, threaded=True)
