from enum import Enum


class DeviceStatus(str, Enum):
    online = "online"
    offline = "offline"


class AccountStatus(str, Enum):
    active = "active"
    invalid = "invalid"
    verifying = "verifying"


class TaskStatus(str, Enum):
    pending = "pending"
    running = "running"
    success = "success"
    failed = "failed"
    needs_verify = "needs_verify"

