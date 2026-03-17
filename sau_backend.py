import asyncio
import json
import os
import secrets
import sqlite3
import threading
import time
import uuid
import socket
from urllib import error as urllib_error
from urllib import request as urllib_request
from pathlib import Path
from queue import Empty, Queue
import conf as app_conf
from myUtils.auth import check_cookie
from flask import Flask, request, jsonify, Response, render_template, send_from_directory
from conf import BASE_DIR
from myUtils.login import get_tencent_cookie, douyin_cookie_gen, get_ks_cookie, xiaohongshu_cookie_gen
from utils.cloud_agent import CloudAgent
from utils.cloud_qr_bridge import CloudLoginBridge
from utils.cloud_sync import CloudSyncClient
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
from utils.publish_task_manager import PublishTaskManager

active_queues = {}
app = Flask(__name__)
PLATFORM_LABELS = {
    '1': '小红书',
    '2': '视频号',
    '3': '抖音',
    '4': '快手'
}


def parse_bool(value):
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    if isinstance(value, (int, float)):
        return value != 0
    return str(value).strip().lower() in {'1', 'true', 'yes', 'on'}


def parse_csv(value, default=None):
    if value is None:
        return list(default or [])
    if isinstance(value, (list, tuple, set)):
        return [str(item).strip() for item in value if str(item).strip()]
    parts = [item.strip() for item in str(value).split(',')]
    cleaned = [item for item in parts if item]
    return cleaned or list(default or [])


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
OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL', 30))
OMNIDRIVE_ACCOUNT_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_ACCOUNT_SYNC_INTERVAL', 60))
OMNIDRIVE_MATERIAL_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_MATERIAL_SYNC_INTERVAL', 300))
OMNIDRIVE_SKILL_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_SKILL_SYNC_INTERVAL', 120))
OMNIDRIVE_PUBLISH_SYNC_INTERVAL = int(getattr(app_conf, 'OMNIDRIVE_PUBLISH_SYNC_INTERVAL', 5))
OMNIDRIVE_MATERIAL_SYNC_MAX_FILES = int(getattr(app_conf, 'OMNIDRIVE_MATERIAL_SYNC_MAX_FILES', 1000))
OMNIBULL_PUBLISH_WORKERS = int(getattr(app_conf, 'OMNIBULL_PUBLISH_WORKERS', 3))
OMNIBULL_TASK_RETENTION_DAYS = int(getattr(app_conf, 'OMNIBULL_TASK_RETENTION_DAYS', 7))
OMNIBULL_API_KEY = str(getattr(app_conf, 'OMNIBULL_API_KEY', '')).strip()
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
RESOLVED_DEVICE_NAME = CLOUD_DEVICE_NAME or socket.gethostname()
DEVICE_CODE = str(getattr(app_conf, 'CLOUD_DEVICE_CODE', '')).strip() or get_device_code()
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
)

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
    cookie_path = Path(BASE_DIR / "cookiesFile" / row['filePath'])
    platform_type = int(row['type'])
    return {
        "id": row['id'],
        "platformType": platform_type,
        "platformName": PLATFORM_LABELS.get(str(platform_type), "未知平台"),
        "filePath": row['filePath'],
        "cookieFilePath": row['filePath'],
        "cookieAbsolutePath": str(cookie_path.resolve()),
        "userName": row['userName'],
        "status": row_status,
        "cookieExists": cookie_path.exists(),
    }


async def validate_account_rows(conn, rows):
    validated_rows = []
    updates = []

    for row in rows:
        is_valid = await check_cookie(row['type'], row['filePath'])
        status = 1 if is_valid else 0
        validated_rows.append(serialize_account_row(row, status))
        if row['status'] != status:
            updates.append((status, row['id']))

    if updates:
        cursor = conn.cursor()
        cursor.executemany(
            '''
            UPDATE user_info
            SET status = ?
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


def ensure_skill_api_authorized():
    if not OMNIBULL_API_KEY:
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


def fetch_account_rows_by_file_paths(account_file_paths):
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
        "materialRoots": list_material_roots(OMNIBULL_MATERIAL_ROOTS),
        "skillApiAuthEnabled": bool(OMNIBULL_API_KEY),
        "cloudAgentConfig": get_cloud_agent_config(),
        "cloudAgent": agent_status,
        "omniDriveAgentConfig": get_omnidrive_agent_config(),
        "omniDriveAgent": omnidrive_agent_status,
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
        bridge.push_log("本地浏览器已启动，等待二维码...")

        while True:
            msg = status_queue.get(timeout=240)

            if msg == "200":
                bridge.push_login_success()
                break

            if msg == "500":
                bridge.push_login_failed()
                break

            if isinstance(msg, dict):
                event_type = msg.get("type")
                payload = msg.get("payload") or {}

                if event_type == "qr_ready" and payload.get("qrData"):
                    bridge.push_qr(payload["qrData"])
                    bridge.push_log(payload.get("message") or "二维码已就绪，请在远端页面扫码")
                    continue

                if event_type == "verification_required":
                    bridge.push_verification(payload)
                    if payload.get("message"):
                        bridge.push_log(payload["message"])
                    continue

                if event_type == "log" and payload.get("message"):
                    bridge.push_log(payload["message"])
                    continue

            if isinstance(msg, str) and msg:
                bridge.push_qr(msg)
                bridge.push_log("二维码已就绪，请在远端页面扫码")
    except Empty:
        try:
            bridge.push_login_failed("本地登录超时，未等到扫码完成")
        except Exception:
            pass
    except Exception as exc:
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
        "heartbeatInterval": OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL,
        "accountSyncInterval": OMNIDRIVE_ACCOUNT_SYNC_INTERVAL,
        "materialSyncInterval": OMNIDRIVE_MATERIAL_SYNC_INTERVAL,
        "skillSyncInterval": OMNIDRIVE_SKILL_SYNC_INTERVAL,
        "publishSyncInterval": OMNIDRIVE_PUBLISH_SYNC_INTERVAL,
        "maxMaterialFiles": OMNIDRIVE_MATERIAL_SYNC_MAX_FILES,
        "startEligible": blocked_reason is None,
        "blockedReason": blocked_reason,
    }


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
            return response.status, json.loads(payload) if payload else {}
    except urllib_error.HTTPError as exc:
        payload = exc.read().decode("utf-8")
        try:
            parsed = json.loads(payload) if payload else {}
        except json.JSONDecodeError:
            parsed = {"error": payload or str(exc)}
        return exc.code, parsed


def ensure_publish_task_manager_started():
    publish_task_manager.start()


def ensure_omnidrive_ai_task_manager_started():
    omnidrive_ai_task_manager.start()


def ensure_cloud_agent_started():
    global cloud_agent

    if not CLOUD_AGENT_ENABLED or not CLOUD_DEMO_URL or not CLOUD_AGENT_KEY:
        return

    with cloud_agent_lock:
        if cloud_agent is None:
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
                generated_root_name=OMNIBULL_GENERATED_ROOT_NAME,
                generated_root_path=OMNIBULL_GENERATED_ROOT_PATH,
                poll_interval=OMNIDRIVE_AGENT_POLL_INTERVAL,
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


@app.route("/getValidAccounts",methods=['GET'])
async def getValidAccounts():
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

            # 删除关联的cookie文件
            if record.get('filePath'):
                cookie_file_path = Path(BASE_DIR / "cookiesFile" / record['filePath'])
                if cookie_file_path.exists():
                    try:
                        cookie_file_path.unlink()
                        print(f"✅ Cookie文件已删除: {cookie_file_path}")
                    except Exception as e:
                        print(f"⚠️ 删除Cookie文件失败: {e}")

            # 删除数据库记录
            cursor.execute("DELETE FROM user_info WHERE id = ?", (account_id,))
            conn.commit()

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


# SSE 登录接口
@app.route('/login')
def login():
    # 1 小红书 2 视频号 3 抖音 4 快手
    type = request.args.get('type')
    # 账号名
    id = request.args.get('id')

    # 模拟一个用于异步通信的队列
    status_queue = Queue()
    active_queues[id] = status_queue

    def on_close():
        print(f"清理队列: {id}")
        del active_queues[id]
    # 启动异步任务线程
    thread = threading.Thread(target=run_async_function, args=(type,id,status_queue), daemon=True)
    thread.start()
    response = Response(sse_stream(status_queue,), mimetype='text/event-stream')
    response.headers['Cache-Control'] = 'no-cache'
    response.headers['X-Accel-Buffering'] = 'no'  # 关键：禁用 Nginx 缓冲
    response.headers['Content-Type'] = 'text/event-stream'
    response.headers['Connection'] = 'keep-alive'
    return response


@app.route('/remoteLogin', methods=['GET', 'POST'])
def remote_login():
    data = get_request_payload()
    type = str(data.get('type', '')).strip()
    account_name = str(data.get('id') or data.get('accountName') or '').strip()
    cloud_url = str(data.get('cloudUrl') or '').strip()
    device_name = str(data.get('deviceName') or '').strip() or None
    platform_name = PLATFORM_LABELS.get(type)

    if not platform_name:
        return jsonify({
            "code": 400,
            "msg": "不支持的平台类型",
            "data": None
        }), 400

    if not account_name:
        return jsonify({
            "code": 400,
            "msg": "账号名称不能为空",
            "data": None
        }), 400

    if not cloud_url:
        return jsonify({
            "code": 400,
            "msg": "cloudUrl 不能为空",
            "data": None
        }), 400

    try:
        bridge = CloudLoginBridge(cloud_url, platform_name, account_name, device_name=device_name)
        session_data = bridge.create_session()
        status_queue = Queue()
        command_queue = Queue()
        action_stop_event = threading.Event()

        threading.Thread(
            target=run_async_function,
            args=(type, account_name, status_queue, command_queue),
            daemon=True
        ).start()
        threading.Thread(
            target=bridge.poll_actions_loop,
            args=(command_queue, action_stop_event),
            daemon=True
        ).start()

        def relay_and_stop():
            try:
                relay_remote_login_status(status_queue, bridge)
            finally:
                action_stop_event.set()

        threading.Thread(target=relay_and_stop, daemon=True).start()

        return jsonify({
            "code": 200,
            "msg": "远端扫码登录已启动",
            "data": session_data
        }), 200
    except Exception as exc:
        return jsonify({
            "code": 500,
            "msg": f"启动远端扫码登录失败: {exc}",
            "data": None
        }), 500


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


def create_local_ai_task(data, source="local_ui"):
    ensure_omnidrive_ai_task_manager_started()
    task = omnidrive_ai_task_manager.create_task(data, source=source)
    return task


@app.route('/aiTasks', methods=['GET', 'POST'])
def local_ai_tasks():
    if request.method == 'POST':
        try:
            task = create_local_ai_task(request.get_json(silent=True) or {}, source="local_ui")
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
        return jsonify({
            "code": status_code,
            "msg": message or "获取 OmniDrive 设备会话失败",
            "data": payload if isinstance(payload, dict) else None,
        }), status_code

    return jsonify({
        "code": 200,
        "msg": "success",
        "data": payload,
    }), 200


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
    data_list = request.get_json()

    if not isinstance(data_list, list):
        return jsonify({"code": 400, "msg": "Expected a JSON array", "data": None}), 400
    try:
        ensure_publish_task_manager_started()
        all_tasks = []
        for data in data_list:
            print("File List:", data.get('fileList', []))
            print("Account List:", data.get('accountList', []))
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

# Cookie文件上传API
@app.route('/uploadCookie', methods=['POST'])
def upload_cookie():
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
        platform = request.form.get('platform')

        if not account_id or not platform:
            return jsonify({
                "code": 400,
                "msg": "缺少账号ID或平台信息",
                "data": None
            }), 400

        # 从数据库获取账号的文件路径
        with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute('SELECT filePath FROM user_info WHERE id = ?', (account_id,))
            result = cursor.fetchone()

        if not result:
            return jsonify({
                "code": 500,
                "msg": "账号不存在",
                "data": None
            }), 404

        # 保存上传的Cookie文件到对应路径
        cookie_file_path = Path(BASE_DIR / "cookiesFile" / result['filePath'])
        cookie_file_path.parent.mkdir(parents=True, exist_ok=True)

        file.save(str(cookie_file_path))

        # 更新数据库中的账号信息（可选，比如更新更新时间）
        # 这里可以根据需要添加额外的处理逻辑

        return jsonify({
            "code": 200,
            "msg": "Cookie文件上传成功",
            "data": None
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
    try:
        file_path = request.args.get('filePath')
        if not file_path:
            return jsonify({
                "code": 500,
                "msg": "缺少文件路径参数",
                "data": None
            }), 400

        # 验证文件路径的安全性，防止路径遍历攻击
        cookie_file_path = Path(BASE_DIR / "cookiesFile" / file_path).resolve()
        base_path = Path(BASE_DIR / "cookiesFile").resolve()

        if not cookie_file_path.is_relative_to(base_path):
            return jsonify({
                "code": 500,
                "msg": "非法文件路径",
                "data": None
            }), 400

        if not cookie_file_path.exists():
            return jsonify({
                "code": 500,
                "msg": "Cookie文件不存在",
                "data": None
            }), 404

        # 返回文件
        return send_from_directory(
            directory=str(cookie_file_path.parent),
            path=cookie_file_path.name,
            as_attachment=True
        )

    except Exception as e:
        print(f"下载Cookie文件时出错: {str(e)}")
        return jsonify({
            "code": 500,
            "msg": f"下载Cookie文件失败: {str(e)}",
            "data": None
        }), 500


# 包装函数：在线程中运行异步函数
def run_async_function(type,id,status_queue,command_queue=None):
    try:
        match type:
            case '1':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(xiaohongshu_cookie_gen(id, status_queue, command_queue))
                loop.close()
            case '2':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(get_tencent_cookie(id,status_queue, command_queue))
                loop.close()
            case '3':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(douyin_cookie_gen(id,status_queue, command_queue))
                loop.close()
            case '4':
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)
                loop.run_until_complete(get_ks_cookie(id,status_queue, command_queue))
                loop.close()
            case _:
                raise ValueError(f"unsupported login type: {type}")
    except Exception as exc:
        print(f"登录线程执行失败: {exc}")
        if status_queue is not None:
            status_queue.put("500")


if should_boot_background_services():
    ensure_publish_task_manager_started()
    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()

# SSE 流生成器函数
def sse_stream(status_queue):
    while True:
        if not status_queue.empty():
            msg = status_queue.get()
            yield f"data: {msg}\n\n"
        else:
            # 避免 CPU 占满
            time.sleep(0.1)

if __name__ == '__main__':
    ensure_publish_task_manager_started()
    ensure_cloud_agent_started()
    ensure_omnidrive_agent_started()
    app.run(host='0.0.0.0' ,port=5409)
