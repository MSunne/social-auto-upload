import socket
import threading
import time
from queue import Queue

import requests

from utils.cloud_qr_bridge import CloudLoginBridge
from utils.device_meta import get_device_code, get_local_ip


class CloudAgent:
    def __init__(
        self,
        cloud_base_url,
        agent_key,
        run_login_fn,
        relay_fn,
        device_name=None,
        poll_interval=5,
        heartbeat_interval=30,
        device_code=None,
    ):
        self.cloud_base_url = cloud_base_url.rstrip('/')
        self.agent_key = agent_key
        self.run_login_fn = run_login_fn
        self.relay_fn = relay_fn
        self.device_name = device_name or socket.gethostname()
        self.poll_interval = max(2, int(poll_interval))
        self.heartbeat_interval = max(10, int(heartbeat_interval))
        self.device_code = device_code or get_device_code()

        self._thread = None
        self._thread_lock = threading.Lock()
        self._busy = threading.Event()
        self._stop = threading.Event()
        self._state_lock = threading.Lock()
        self._state = {
            "running": False,
            "busy": False,
            "deviceName": self.device_name,
            "deviceCode": self.device_code,
            "cloudUrl": self.cloud_base_url,
            "localIp": get_local_ip(),
            "lastHeartbeatAt": None,
            "lastTaskAt": None,
            "lastError": None,
            "currentTask": None
        }

    def start(self):
        with self._thread_lock:
            if self._thread and self._thread.is_alive():
                return

            self._stop.clear()
            self._thread = threading.Thread(target=self._loop, daemon=True)
            self._thread.start()

    def status(self):
        with self._state_lock:
            return dict(self._state)

    def _update_state(self, **kwargs):
        with self._state_lock:
            self._state.update(kwargs)

    def _agent_payload(self):
        return {
            "deviceName": self.device_name,
            "agentKey": self.agent_key,
            "deviceCode": self.device_code,
            "localIp": get_local_ip(),
        }

    def _loop(self):
        self._update_state(running=True, lastError=None)
        last_heartbeat_at = 0.0

        while not self._stop.is_set():
            try:
                now = time.monotonic()
                if now - last_heartbeat_at >= self.heartbeat_interval:
                    self._heartbeat()
                    last_heartbeat_at = now

                if not self._busy.is_set():
                    task = self._claim_task()
                    if task:
                        self._busy.set()
                        self._update_state(
                            busy=True,
                            currentTask={
                                "taskId": task["taskId"],
                                "platform": task["platform"],
                                "accountName": task["accountName"]
                            },
                            lastTaskAt=time.strftime("%Y-%m-%d %H:%M:%S")
                        )
                        threading.Thread(target=self._execute_task, args=(task,), daemon=True).start()
            except Exception as exc:
                self._update_state(lastError=str(exc))

            time.sleep(self.poll_interval)

    def _heartbeat(self):
        response = requests.post(
            f"{self.cloud_base_url}/api/agents/heartbeat",
            json=self._agent_payload(),
            timeout=10
        )
        response.raise_for_status()
        result = response.json()
        if result.get("code") != 200:
            raise RuntimeError(result.get("msg") or "agent heartbeat failed")

        self._update_state(
            lastHeartbeatAt=time.strftime("%Y-%m-%d %H:%M:%S"),
            localIp=get_local_ip(),
            lastError=None,
        )

    def _claim_task(self):
        response = requests.post(
            f"{self.cloud_base_url}/api/agents/next-task",
            json=self._agent_payload(),
            timeout=10
        )
        response.raise_for_status()
        result = response.json()
        if result.get("code") != 200:
            raise RuntimeError(result.get("msg") or "claim task failed")
        return result.get("data")

    def _execute_task(self, task):
        bridge = None
        action_stop_event = threading.Event()
        try:
            bridge = CloudLoginBridge(
                self.cloud_base_url,
                task["platform"],
                task["accountName"],
                device_name=self.device_name
            )
            bridge.attach_session(
                session_id=task["sessionId"],
                writer_token=task["writerToken"],
                viewer_token=task.get("viewerToken"),
                viewer_url=task.get("viewerUrl")
            )

            status_queue = Queue()
            command_queue = Queue()
            threading.Thread(
                target=bridge.poll_actions_loop,
                args=(command_queue, action_stop_event),
                daemon=True
            ).start()
            threading.Thread(
                target=self.run_login_fn,
                args=(str(task["platformType"]), task["accountName"], status_queue, command_queue),
                daemon=True
            ).start()
            self.relay_fn(status_queue, bridge)
        except Exception as exc:
            if bridge is not None:
                try:
                    bridge.push_login_failed(f"本地 agent 执行任务失败: {exc}")
                except Exception:
                    pass
            self._update_state(lastError=str(exc))
        finally:
            action_stop_event.set()
            self._busy.clear()
            self._update_state(busy=False, currentTask=None)
