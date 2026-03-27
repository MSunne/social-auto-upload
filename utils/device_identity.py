import json
from pathlib import Path

from utils.device_meta import get_device_code


def _clean_string(value):
    if value is None:
        return ""
    return str(value).strip()


def _load_identity_payload(path):
    if not path:
        return {}

    identity_path = Path(path).expanduser()
    if not identity_path.is_absolute():
        identity_path = identity_path.resolve()
    if not identity_path.exists() or not identity_path.is_file():
        return {}

    try:
        payload = json.loads(identity_path.read_text(encoding="utf-8"))
    except Exception:
        return {}
    return payload if isinstance(payload, dict) else {}


def load_device_identity(app_conf, base_dir):
    configured_path = _clean_string(getattr(app_conf, "OMNIBULL_DEVICE_IDENTITY_FILE", ""))
    if configured_path:
        payload = _load_identity_payload(configured_path)
        if payload:
            return {
                "path": str(Path(configured_path).expanduser()),
                "source": "device_identity_file",
                "deviceCode": _clean_string(payload.get("deviceCode")),
                "agentKey": _clean_string(payload.get("agentKey")),
                "deviceName": _clean_string(payload.get("deviceName")),
                "localApiKey": _clean_string(payload.get("localApiKey")),
            }

    return {
        "path": None,
        "source": "conf_or_runtime_fallback",
        "deviceCode": _clean_string(getattr(app_conf, "CLOUD_DEVICE_CODE", "")) or get_device_code(),
        "agentKey": _clean_string(getattr(app_conf, "OMNIDRIVE_AGENT_KEY", "")),
        "deviceName": _clean_string(getattr(app_conf, "CLOUD_DEVICE_NAME", "")),
        "localApiKey": _clean_string(getattr(app_conf, "OMNIBULL_API_KEY", "")),
    }
