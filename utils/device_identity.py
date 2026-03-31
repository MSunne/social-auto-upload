import json
import os
import secrets
import socket
import sys
from datetime import datetime, timezone
from pathlib import Path

from utils.device_meta import get_device_fingerprint, get_stable_device_code

AUTO_PROVISION_METHOD = "auto_bootstrap"
LEGACY_RUNTIME_IDENTITY_PATH = Path("runtime").joinpath("device.identity.json")


def _clean_string(value):
    if value is None:
        return ""
    return str(value).strip()


def _resolve_identity_path(path, base_dir):
    identity_path = Path(path).expanduser()
    if identity_path.is_absolute():
        return identity_path
    return Path(base_dir).joinpath(identity_path).resolve()


def _serialize_payload(payload):
    return json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n"


def _load_identity_payload(path):
    if not path.exists() or not path.is_file():
        return {}

    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return {}
    return payload if isinstance(payload, dict) else {}


def _write_identity_payload(path, payload):
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(_serialize_payload(payload), encoding="utf-8")
        try:
            os.chmod(path, 0o600)
        except OSError:
            pass
        return True
    except OSError:
        return False


def _now_iso():
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


def _build_runtime_defaults(app_conf):
    device_name = (
        _clean_string(getattr(app_conf, "OMNIBULL_DEVICE_NAME", ""))
        or _clean_string(getattr(app_conf, "OMNIDRIVE_DEVICE_NAME", ""))
        or socket.gethostname()
    )
    return {
        "deviceCode": (
            _clean_string(getattr(app_conf, "OMNIBULL_DEVICE_CODE", ""))
            or _clean_string(getattr(app_conf, "OMNIDRIVE_DEVICE_CODE", ""))
            or get_stable_device_code()
        ),
        "agentKey": _clean_string(getattr(app_conf, "OMNIDRIVE_AGENT_KEY", "")),
        "deviceName": device_name,
        "localApiKey": _clean_string(getattr(app_conf, "OMNIBULL_API_KEY", "")),
        "deviceFingerprint": get_device_fingerprint(),
    }


def _build_auto_payload(defaults):
    return {
        "identityVersion": 2,
        "provisionMethod": AUTO_PROVISION_METHOD,
        "provisionedAt": _now_iso(),
        "deviceCode": defaults["deviceCode"],
        "agentKey": defaults["agentKey"] or secrets.token_hex(32),
        "deviceName": defaults["deviceName"],
        "localApiKey": defaults["localApiKey"],
        "deviceFingerprint": defaults["deviceFingerprint"],
    }


def _normalize_payload(payload, defaults):
    normalized = {
        "identityVersion": int(payload.get("identityVersion") or 1),
        "provisionMethod": _clean_string(payload.get("provisionMethod")) or AUTO_PROVISION_METHOD,
        "provisionedAt": _clean_string(payload.get("provisionedAt")) or _now_iso(),
        "deviceCode": _clean_string(payload.get("deviceCode")) or defaults["deviceCode"],
        "agentKey": _clean_string(payload.get("agentKey")) or defaults["agentKey"] or secrets.token_hex(32),
        "deviceName": _clean_string(payload.get("deviceName")) or defaults["deviceName"],
        "localApiKey": _clean_string(payload.get("localApiKey")) or defaults["localApiKey"],
        "deviceFingerprint": _clean_string(payload.get("deviceFingerprint")) or defaults["deviceFingerprint"],
    }
    current_fingerprint = _clean_string(defaults.get("deviceFingerprint"))
    if current_fingerprint and normalized["deviceFingerprint"] != current_fingerprint:
        normalized["deviceFingerprint"] = current_fingerprint
    return normalized


def _public_identity(path, payload, source):
    return {
        "path": str(path) if path else None,
        "source": source,
        "deviceCode": payload["deviceCode"],
        "agentKey": payload["agentKey"],
        "deviceName": payload["deviceName"],
        "localApiKey": payload["localApiKey"],
        "deviceFingerprint": payload.get("deviceFingerprint"),
    }


def _default_identity_path(base_dir):
    system_path = Path("/etc/omnibull/device.json")
    if system_path.exists():
        return system_path

    home_dir = Path.home()
    if os.name == "nt":
        for env_name in ("PROGRAMDATA", "APPDATA"):
            root = _clean_string(os.environ.get(env_name))
            if root:
                return Path(root).expanduser().joinpath("OmniBull", "device.json")
        return home_dir.joinpath("AppData", "Roaming", "OmniBull", "device.json")

    if sys.platform == "darwin":
        return home_dir.joinpath("Library", "Application Support", "OmniBull", "device.json")

    state_root = _clean_string(os.environ.get("XDG_STATE_HOME"))
    if state_root:
        return Path(state_root).expanduser().joinpath("omnibull", "device.json")
    if str(home_dir).strip() and str(home_dir) != "/":
        return home_dir.joinpath(".local", "state", "omnibull", "device.json")
    return Path(base_dir).joinpath(LEGACY_RUNTIME_IDENTITY_PATH).resolve()


def _primary_identity_path(configured_path, base_dir):
    if configured_path:
        return _resolve_identity_path(configured_path, base_dir)
    return _default_identity_path(base_dir).resolve()


def _legacy_identity_paths(primary_path, base_dir):
    legacy_path = Path(base_dir).joinpath(LEGACY_RUNTIME_IDENTITY_PATH).resolve()
    if legacy_path == primary_path:
        return []
    return [legacy_path]


def load_device_identity(app_conf, base_dir):
    configured_path = _clean_string(getattr(app_conf, "OMNIBULL_DEVICE_IDENTITY_FILE", ""))
    defaults = _build_runtime_defaults(app_conf)
    primary_path = _primary_identity_path(configured_path, base_dir)
    legacy_paths = _legacy_identity_paths(primary_path, base_dir)
    candidate_paths = [primary_path, *legacy_paths]

    for path in candidate_paths:
        payload = _load_identity_payload(path)
        if not payload:
            continue

        normalized = _normalize_payload(payload, defaults)
        source = "device_identity_file"
        target_path = path

        if path != primary_path:
            source = "device_identity_file_migrated"
            target_path = primary_path
            if _write_identity_payload(primary_path, normalized):
                return _public_identity(primary_path, normalized, source)
            target_path = path
            source = "device_identity_file"

        if _serialize_payload(payload) != _serialize_payload(normalized) and _write_identity_payload(target_path, normalized):
            return _public_identity(target_path, normalized, "device_identity_file_updated")
        return _public_identity(target_path, normalized, source)

    auto_payload = _build_auto_payload(defaults)
    for path in [primary_path, *legacy_paths]:
        if _write_identity_payload(path, auto_payload):
            return _public_identity(path, auto_payload, "device_identity_file_created")

    return _public_identity(None, auto_payload, "device_identity_runtime_fallback")
