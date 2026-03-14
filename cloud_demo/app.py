import json
import os
import sqlite3
import threading
import uuid
from datetime import datetime, timedelta
from pathlib import Path
from queue import Empty, Queue

from flask import Flask, Response, jsonify, redirect, render_template, request, url_for


BASE_DIR = Path(__file__).resolve().parent
DB_PATH = Path(os.getenv("CLOUD_QR_DEMO_DB", BASE_DIR / "cloud_demo.db"))

app = Flask(__name__, template_folder=str(BASE_DIR / "templates"))
session_subscribers = {}
subscribers_lock = threading.Lock()
PLATFORM_TYPE_MAP = {
    "小红书": 1,
    "视频号": 2,
    "抖音": 3,
    "快手": 4
}


def get_db_connection():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db():
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            CREATE TABLE IF NOT EXISTS login_sessions (
                session_id TEXT PRIMARY KEY,
                viewer_token TEXT NOT NULL UNIQUE,
                writer_token TEXT NOT NULL,
                platform TEXT NOT NULL,
                account_name TEXT NOT NULL,
                device_name TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'pending',
                qr_data TEXT,
                message TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )
            '''
        )
        cursor.execute(
            '''
            CREATE TABLE IF NOT EXISTS account_mirror (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                device_name TEXT NOT NULL,
                platform TEXT NOT NULL,
                account_name TEXT NOT NULL,
                status TEXT NOT NULL,
                last_message TEXT,
                last_session_id TEXT,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                UNIQUE(device_name, platform, account_name)
            )
            '''
        )
        cursor.execute(
            '''
            CREATE TABLE IF NOT EXISTS agent_devices (
                device_name TEXT PRIMARY KEY,
                agent_key TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'offline',
                last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
            )
            '''
        )
        cursor.execute(
            '''
            CREATE TABLE IF NOT EXISTS login_tasks (
                task_id TEXT PRIMARY KEY,
                device_name TEXT NOT NULL,
                platform TEXT NOT NULL,
                platform_type INTEGER NOT NULL,
                account_name TEXT NOT NULL,
                session_id TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'pending',
                last_error TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                claimed_at DATETIME,
                finished_at DATETIME
            )
            '''
        )
        conn.commit()


def serialize_session(row):
    if row is None:
        return None
    return {
        "sessionId": row["session_id"],
        "viewerToken": row["viewer_token"],
        "platform": row["platform"],
        "accountName": row["account_name"],
        "deviceName": row["device_name"],
        "status": row["status"],
        "qrData": row["qr_data"],
        "message": row["message"],
        "createdAt": row["created_at"],
        "updatedAt": row["updated_at"]
    }


def is_device_online(last_seen):
    if not last_seen:
        return False

    try:
        last_seen_dt = datetime.fromisoformat(last_seen)
    except ValueError:
        return False

    return datetime.utcnow() - last_seen_dt <= timedelta(seconds=20)


def serialize_device(row):
    online = is_device_online(row["last_seen"])
    return {
        "deviceName": row["device_name"],
        "status": "online" if online else "offline",
        "lastSeen": row["last_seen"],
        "createdAt": row["created_at"],
        "updatedAt": row["updated_at"]
    }


def create_session_record(platform, account_name, device_name):
    session_id = uuid.uuid4().hex
    viewer_token = uuid.uuid4().hex
    writer_token = uuid.uuid4().hex

    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            INSERT INTO login_sessions (
                session_id, viewer_token, writer_token, platform, account_name, device_name, status, message
            )
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            ''',
            (
                session_id,
                viewer_token,
                writer_token,
                platform,
                account_name,
                device_name,
                "pending",
                "等待本地 agent 启动浏览器"
            )
        )
        conn.commit()

    return {
        "sessionId": session_id,
        "viewerToken": viewer_token,
        "writerToken": writer_token
    }


def create_login_task(device_name, platform, account_name):
    platform_type = PLATFORM_TYPE_MAP.get(platform)
    if not platform_type:
        raise ValueError("unsupported platform")

    session = create_session_record(platform, account_name, device_name)
    task_id = uuid.uuid4().hex

    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            INSERT INTO login_tasks (
                task_id, device_name, platform, platform_type, account_name, session_id, status
            )
            VALUES (?, ?, ?, ?, ?, ?, ?)
            ''',
            (
                task_id,
                device_name,
                platform,
                platform_type,
                account_name,
                session["sessionId"],
                "pending"
            )
        )
        conn.commit()

    return {
        "taskId": task_id,
        "deviceName": device_name,
        "platform": platform,
        "platformType": platform_type,
        "accountName": account_name,
        **session
    }


def load_session_by_id(session_id):
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM login_sessions
            WHERE session_id = ?
            ''',
            (session_id,)
        )
        return cursor.fetchone()


def load_session_by_viewer_token(viewer_token):
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM login_sessions
            WHERE viewer_token = ?
            ''',
            (viewer_token,)
        )
        return cursor.fetchone()


def load_device(device_name):
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM agent_devices
            WHERE device_name = ?
            ''',
            (device_name,)
        )
        return cursor.fetchone()


def publish_session_event(session_id, event):
    with subscribers_lock:
        subscribers = list(session_subscribers.get(session_id, []))

    for subscriber in subscribers:
        subscriber.put(event)


def add_subscriber(session_id):
    queue = Queue()
    with subscribers_lock:
        session_subscribers.setdefault(session_id, []).append(queue)
    return queue


def remove_subscriber(session_id, queue):
    with subscribers_lock:
        subscribers = session_subscribers.get(session_id, [])
        if queue in subscribers:
            subscribers.remove(queue)
        if not subscribers and session_id in session_subscribers:
            del session_subscribers[session_id]


def upsert_account_mirror(cursor, session_row, status, message):
    cursor.execute(
        '''
        INSERT INTO account_mirror (
            device_name, platform, account_name, status, last_message, last_session_id, updated_at
        )
        VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
        ON CONFLICT(device_name, platform, account_name)
        DO UPDATE SET
            status = excluded.status,
            last_message = excluded.last_message,
            last_session_id = excluded.last_session_id,
            updated_at = CURRENT_TIMESTAMP
        ''',
        (
            session_row["device_name"],
            session_row["platform"],
            session_row["account_name"],
            status,
            message,
            session_row["session_id"]
        )
    )


def update_task_status(cursor, session_id, status, error=None, finished=False):
    if finished:
        cursor.execute(
            '''
            UPDATE login_tasks
            SET status = ?, last_error = ?, finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
            WHERE session_id = ?
            ''',
            (status, error, session_id)
        )
    else:
        cursor.execute(
            '''
            UPDATE login_tasks
            SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
            WHERE session_id = ?
            ''',
            (status, error, session_id)
        )


def upsert_agent_device(device_name, agent_key):
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM agent_devices
            WHERE device_name = ?
            ''',
            (device_name,)
        )
        row = cursor.fetchone()

        if row and row["agent_key"] != agent_key:
            raise PermissionError("agent key mismatch")

        cursor.execute(
            '''
            INSERT INTO agent_devices (device_name, agent_key, status, last_seen, updated_at)
            VALUES (?, ?, 'online', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
            ON CONFLICT(device_name)
            DO UPDATE SET
                status = 'online',
                last_seen = CURRENT_TIMESTAMP,
                updated_at = CURRENT_TIMESTAMP
            ''',
            (device_name, agent_key)
        )
        conn.commit()


def apply_session_event(session_row, event_type, payload):
    next_status = session_row["status"]
    next_qr_data = session_row["qr_data"]
    next_message = session_row["message"]

    with get_db_connection() as conn:
        cursor = conn.cursor()

        if event_type == "qr_ready":
            next_status = "waiting_scan"
            next_qr_data = payload.get("qrData")
            next_message = payload.get("message") or "二维码已就绪，请使用手机扫码登录"
            upsert_account_mirror(cursor, session_row, "等待扫码", next_message)
            update_task_status(cursor, session_row["session_id"], "running")
        elif event_type == "login_success":
            next_status = "success"
            next_message = payload.get("message") or "扫码登录成功，本地 token 已保存"
            upsert_account_mirror(cursor, session_row, "正常", next_message)
            update_task_status(cursor, session_row["session_id"], "success", finished=True)
        elif event_type == "login_failed":
            next_status = "failed"
            next_message = payload.get("message") or "扫码登录失败或超时"
            upsert_account_mirror(cursor, session_row, "异常", next_message)
            update_task_status(cursor, session_row["session_id"], "failed", error=next_message, finished=True)
        elif event_type == "log":
            next_message = payload.get("message") or next_message
        else:
            raise ValueError(f"unsupported event type: {event_type}")

        cursor.execute(
            '''
            UPDATE login_sessions
            SET status = ?, qr_data = ?, message = ?, updated_at = CURRENT_TIMESTAMP
            WHERE session_id = ?
            ''',
            (next_status, next_qr_data, next_message, session_row["session_id"])
        )
        conn.commit()

    return load_session_by_id(session_row["session_id"])


def collect_dashboard_snapshot():
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM agent_devices
            ORDER BY updated_at DESC, created_at DESC
            '''
        )
        devices = [serialize_device(row) for row in cursor.fetchall()]
        cursor.execute(
            '''
            SELECT * FROM account_mirror
            ORDER BY updated_at DESC, id DESC
            '''
        )
        accounts = [
            {
                "deviceName": row["device_name"],
                "platform": row["platform"],
                "accountName": row["account_name"],
                "status": row["status"],
                "lastMessage": row["last_message"],
                "lastSessionId": row["last_session_id"],
                "updatedAt": row["updated_at"]
            }
            for row in cursor.fetchall()
        ]
        cursor.execute(
            '''
            SELECT t.*, s.viewer_token
            FROM login_tasks t
            LEFT JOIN login_sessions s ON s.session_id = t.session_id
            ORDER BY t.updated_at DESC, t.created_at DESC
            LIMIT 20
            '''
        )
        tasks = [
            {
                "taskId": row["task_id"],
                "deviceName": row["device_name"],
                "platform": row["platform"],
                "accountName": row["account_name"],
                "status": row["status"],
                "updatedAt": row["updated_at"],
                "viewerUrl": request.host_url.rstrip('/') + url_for('view_session', viewer_token=row["viewer_token"])
                if row["viewer_token"] else None
            }
            for row in cursor.fetchall()
        ]

    return {
        "devices": devices,
        "accounts": accounts,
        "tasks": tasks
    }


@app.route('/')
def dashboard():
    snapshot = collect_dashboard_snapshot()
    return render_template('dashboard.html', snapshot=snapshot)


@app.route('/api/accounts')
def api_accounts():
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM account_mirror
            ORDER BY updated_at DESC, id DESC
            '''
        )
        rows = cursor.fetchall()

    data = [
        {
            "deviceName": row["device_name"],
            "platform": row["platform"],
            "accountName": row["account_name"],
            "status": row["status"],
            "lastMessage": row["last_message"],
            "lastSessionId": row["last_session_id"],
            "updatedAt": row["updated_at"]
        }
        for row in rows
    ]
    return jsonify({"code": 200, "msg": "success", "data": data}), 200


@app.route('/api/devices')
def api_devices():
    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT * FROM agent_devices
            ORDER BY updated_at DESC, created_at DESC
            '''
        )
        devices = [serialize_device(row) for row in cursor.fetchall()]

    return jsonify({"code": 200, "msg": "success", "data": devices}), 200


@app.route('/api/dashboard')
def api_dashboard():
    return jsonify({"code": 200, "msg": "success", "data": collect_dashboard_snapshot()}), 200


@app.route('/api/sessions', methods=['POST'])
def create_session():
    payload = request.get_json(silent=True) or {}
    platform = str(payload.get("platform") or "").strip()
    account_name = str(payload.get("accountName") or "").strip()
    device_name = str(payload.get("deviceName") or "unknown-device").strip()

    if not platform or not account_name:
        return jsonify({"code": 400, "msg": "platform 和 accountName 不能为空", "data": None}), 400

    session = create_session_record(platform, account_name, device_name)
    viewer_url = request.host_url.rstrip('/') + url_for('view_session', viewer_token=session["viewerToken"])
    return jsonify({
        "code": 200,
        "msg": "session created",
        "data": {
            "sessionId": session["sessionId"],
            "viewerToken": session["viewerToken"],
            "writerToken": session["writerToken"],
            "viewerUrl": viewer_url
        }
    }), 200


@app.route('/api/agents/heartbeat', methods=['POST'])
def agent_heartbeat():
    payload = request.get_json(silent=True) or {}
    device_name = str(payload.get("deviceName") or "").strip()
    agent_key = str(payload.get("agentKey") or "").strip()

    if not device_name or not agent_key:
        return jsonify({"code": 400, "msg": "deviceName 和 agentKey 不能为空", "data": None}), 400

    try:
        upsert_agent_device(device_name, agent_key)
    except PermissionError as exc:
        return jsonify({"code": 403, "msg": str(exc), "data": None}), 403

    return jsonify({"code": 200, "msg": "heartbeat ok", "data": {"deviceName": device_name}}), 200


@app.route('/api/agents/next-task', methods=['POST'])
def agent_next_task():
    payload = request.get_json(silent=True) or {}
    device_name = str(payload.get("deviceName") or "").strip()
    agent_key = str(payload.get("agentKey") or "").strip()

    if not device_name or not agent_key:
        return jsonify({"code": 400, "msg": "deviceName 和 agentKey 不能为空", "data": None}), 400

    try:
        upsert_agent_device(device_name, agent_key)
    except PermissionError as exc:
        return jsonify({"code": 403, "msg": str(exc), "data": None}), 403

    with get_db_connection() as conn:
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT t.*, s.viewer_token, s.writer_token
            FROM login_tasks t
            JOIN login_sessions s ON s.session_id = t.session_id
            WHERE t.device_name = ? AND t.status = 'pending'
            ORDER BY t.created_at ASC
            LIMIT 1
            ''',
            (device_name,)
        )
        row = cursor.fetchone()

        if not row:
            return jsonify({"code": 200, "msg": "no task", "data": None}), 200

        cursor.execute(
            '''
            UPDATE login_tasks
            SET status = 'running', claimed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
            WHERE task_id = ?
            ''',
            (row["task_id"],)
        )
        conn.commit()

    viewer_url = request.host_url.rstrip('/') + url_for('view_session', viewer_token=row["viewer_token"])
    return jsonify({
        "code": 200,
        "msg": "task claimed",
        "data": {
            "taskId": row["task_id"],
            "deviceName": row["device_name"],
            "platform": row["platform"],
            "platformType": row["platform_type"],
            "accountName": row["account_name"],
            "sessionId": row["session_id"],
            "viewerToken": row["viewer_token"],
            "writerToken": row["writer_token"],
            "viewerUrl": viewer_url
        }
    }), 200


@app.route('/tasks/create', methods=['POST'])
def create_task_from_dashboard():
    device_name = str(request.form.get("deviceName") or "").strip()
    platform = str(request.form.get("platform") or "").strip()
    account_name = str(request.form.get("accountName") or "").strip()

    if not device_name or not platform or not account_name:
        return redirect(url_for('dashboard'))

    task = create_login_task(device_name, platform, account_name)
    return redirect(url_for('view_session', viewer_token=task["viewerToken"]))


@app.route('/api/sessions/<session_id>/events', methods=['POST'])
def session_event(session_id):
    writer_token = request.headers.get("X-Writer-Token", "")
    payload = request.get_json(silent=True) or {}
    event_type = payload.get("eventType")
    event_payload = payload.get("payload") or {}

    session_row = load_session_by_id(session_id)
    if not session_row:
        return jsonify({"code": 404, "msg": "session not found", "data": None}), 404

    if writer_token != session_row["writer_token"]:
        return jsonify({"code": 403, "msg": "invalid writer token", "data": None}), 403

    try:
        updated_session = apply_session_event(session_row, event_type, event_payload)
    except ValueError as exc:
        return jsonify({"code": 400, "msg": str(exc), "data": None}), 400

    event = {
        "type": event_type,
        "session": serialize_session(updated_session)
    }
    publish_session_event(session_id, event)

    return jsonify({"code": 200, "msg": "event accepted", "data": serialize_session(updated_session)}), 200


@app.route('/s/<viewer_token>')
def view_session(viewer_token):
    session_row = load_session_by_viewer_token(viewer_token)
    if not session_row:
        return "session not found", 404

    return render_template(
        'session.html',
        session=serialize_session(session_row),
        stream_url=url_for('session_stream', viewer_token=viewer_token),
        state_url=url_for('session_state', viewer_token=viewer_token)
    )


@app.route('/api/view/<viewer_token>/state')
def session_state(viewer_token):
    session_row = load_session_by_viewer_token(viewer_token)
    if not session_row:
        return jsonify({"code": 404, "msg": "session not found", "data": None}), 404

    return jsonify({"code": 200, "msg": "success", "data": serialize_session(session_row)}), 200


@app.route('/api/view/<viewer_token>/stream')
def session_stream(viewer_token):
    session_row = load_session_by_viewer_token(viewer_token)
    if not session_row:
        return jsonify({"code": 404, "msg": "session not found", "data": None}), 404

    session_id = session_row["session_id"]
    subscriber = add_subscriber(session_id)

    def generate():
        try:
            latest_session = load_session_by_id(session_id)
            yield f"data: {json.dumps({'type': 'snapshot', 'session': serialize_session(latest_session)}, ensure_ascii=False)}\n\n"

            while True:
                try:
                    event = subscriber.get(timeout=25)
                    yield f"data: {json.dumps(event, ensure_ascii=False)}\n\n"
                except Empty:
                    yield "event: ping\ndata: {}\n\n"
        finally:
            remove_subscriber(session_id, subscriber)

    response = Response(generate(), mimetype='text/event-stream')
    response.headers['Cache-Control'] = 'no-cache'
    response.headers['X-Accel-Buffering'] = 'no'
    response.headers['Connection'] = 'keep-alive'
    return response


init_db()


if __name__ == '__main__':
    app.run(host='0.0.0.0', port=int(os.getenv("PORT", "5410")))
