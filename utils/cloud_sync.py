import requests


class CloudSyncClient:
    def __init__(self, cloud_base_url, device_name, agent_key, timeout=10):
        self.cloud_base_url = str(cloud_base_url or "").rstrip("/")
        self.device_name = str(device_name or "").strip()
        self.agent_key = str(agent_key or "").strip()
        self.timeout = timeout

    @property
    def enabled(self):
        return bool(self.cloud_base_url and self.device_name and self.agent_key)

    def sync_publish_task(self, task_payload):
        if not self.enabled:
            return False

        response = requests.post(
            f"{self.cloud_base_url}/api/publish-tasks/sync",
            json={
                "deviceName": self.device_name,
                "agentKey": self.agent_key,
                "task": task_payload,
            },
            timeout=self.timeout,
        )
        response.raise_for_status()
        result = response.json()
        if result.get("code") != 200:
            raise RuntimeError(result.get("msg") or "同步发布任务失败")
        return True
