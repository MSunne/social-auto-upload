import socket
import time

import requests


class CloudLoginBridge:
    def __init__(self, cloud_base_url, platform, account_name, device_name=None, timeout=10):
        self.cloud_base_url = cloud_base_url.rstrip('/')
        self.platform = platform
        self.account_name = account_name
        self.device_name = device_name or socket.gethostname()
        self.timeout = timeout
        self.session_id = None
        self.viewer_token = None
        self.writer_token = None
        self.viewer_url = None

    def attach_session(self, session_id, writer_token, viewer_token=None, viewer_url=None):
        self.session_id = session_id
        self.writer_token = writer_token
        self.viewer_token = viewer_token
        self.viewer_url = viewer_url

    def create_session(self):
        response = requests.post(
            f"{self.cloud_base_url}/api/sessions",
            json={
                "platform": self.platform,
                "accountName": self.account_name,
                "deviceName": self.device_name
            },
            timeout=self.timeout
        )
        response.raise_for_status()
        payload = response.json()

        if payload.get("code") != 200 or not payload.get("data"):
            raise RuntimeError(payload.get("msg") or "创建云端会话失败")

        data = payload["data"]
        self.session_id = data["sessionId"]
        self.viewer_token = data["viewerToken"]
        self.writer_token = data["writerToken"]
        self.viewer_url = data["viewerUrl"]
        return data

    def send_event(self, event_type, payload=None):
        if not self.session_id or not self.writer_token:
            raise RuntimeError("云端会话尚未创建")

        response = requests.post(
            f"{self.cloud_base_url}/api/sessions/{self.session_id}/events",
            json={
                "eventType": event_type,
                "payload": payload or {}
            },
            headers={
                "X-Writer-Token": self.writer_token
            },
            timeout=self.timeout
        )
        response.raise_for_status()
        result = response.json()

        if result.get("code") != 200:
            raise RuntimeError(result.get("msg") or "推送云端事件失败")

    def push_qr(self, qr_data):
        self.send_event("qr_ready", {"qrData": qr_data})

    def push_verification(self, verification_data):
        self.send_event("verification_required", verification_data)

    def push_login_success(self, message="扫码登录成功，本地 token 已保存"):
        self.send_event("login_success", {"message": message})

    def push_login_failed(self, message="扫码登录失败或超时"):
        self.send_event("login_failed", {"message": message})

    def push_log(self, message):
        self.send_event("log", {"message": message})

    def fetch_next_action(self):
        if not self.session_id or not self.writer_token:
            raise RuntimeError("云端会话尚未创建")

        response = requests.get(
            f"{self.cloud_base_url}/api/sessions/{self.session_id}/actions/next",
            headers={
                "X-Writer-Token": self.writer_token
            },
            timeout=self.timeout
        )
        response.raise_for_status()
        payload = response.json()

        if payload.get("code") != 200:
            raise RuntimeError(payload.get("msg") or "获取远端操作失败")

        return payload.get("data")

    def poll_actions_loop(self, command_queue, stop_event, interval=1.2):
        while not stop_event.is_set():
            try:
                action = self.fetch_next_action()
                if action:
                    command_queue.put(action)
            except Exception:
                time.sleep(max(interval, 1.0))
                continue

            time.sleep(max(interval, 0.5))
