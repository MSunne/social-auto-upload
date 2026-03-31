import json
import os
import secrets
import socket
from datetime import datetime, timezone
from pathlib import Path

from utils.device_meta import get_device_code, get_device_fingerprint

AUTO_PROVISION_METHOD = "auto_bootstrap"


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
            or get_device_code()
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
    return {
        "identityVersion": int(payload.get("identityVersion") or 1),
        "provisionMethod": _clean_string(payload.get("provisionMethod")) or AUTO_PROVISION_METHOD,
        "provisionedAt": _clean_string(payload.get("provisionedAt")) or _now_iso(),
        "deviceCode": _clean_string(payload.get("deviceCode")) or defaults["deviceCode"],
        "agentKey": _clean_string(payload.get("agentKey")) or defaults["agentKey"] or secrets.token_hex(32),
        "deviceName": _clean_string(payload.get("deviceName")) or defaults["deviceName"],
        "localApiKey": _clean_string(payload.get("localApiKey")) or defaults["localApiKey"],
        "deviceFingerprint": _clean_string(payload.get("deviceFingerprint")) or defaults["deviceFingerprint"],
    }


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


def _candidate_identity_paths(configured_path, base_dir):
    items = []
    if configured_path:
        items.append(_resolve_identity_path(configured_path, base_dir))
    fallback_path = Path(base_dir).joinpath("runtime", "device.identity.json").resolve()
    if fallback_path not in items:
        items.append(fallback_path)
    return items


def load_device_identity(app_conf, base_dir):
    configured_path = _clean_string(getattr(app_conf, "OMNIBULL_DEVICE_IDENTITY_FILE", ""))
    defaults = _build_runtime_defaults(app_conf)
    current_device_fingerprint = _clean_string(defaults.get("deviceFingerprint"))
    candidate_paths = _candidate_identity_paths(configured_path, base_dir)

    for path in candidate_paths:
        payload = _load_identity_payload(path)
        if not payload:
            continue

        normalized = _normalize_payload(payload, defaults)
        source = "device_identity_file"

        is_auto_provisioned = normalized["provisionMethod"] == AUTO_PROVISION_METHOD
        stored_fingerprint = _clean_string(normalized.get("deviceFingerprint"))
        if is_auto_provisioned and stored_fingerprint and current_device_fingerprint and stored_fingerprint != current_device_fingerprint:
            normalized = _build_auto_payload(defaults)
            normalized["deviceName"] = _clean_string(payload.get("deviceName")) or normalized["deviceName"]
            normalized["localApiKey"] = _clean_string(payload.get("localApiKey")) or normalized["localApiKey"]
            source = "device_identity_file_rotated"
        elif is_auto_provisioned and current_device_fingerprint and not stored_fingerprint:
            normalized["deviceFingerprint"] = current_device_fingerprint
            source = "device_identity_file_updated"
        elif _serialize_payload(payload) != _serialize_payload(normalized):
            source = "device_identity_file_updated"

        if source != "device_identity_file" and _write_identity_payload(path, normalized):
            return _public_identity(path, normalized, source)
        if source == "device_identity_file":
            return _public_identity(path, normalized, source)

    auto_payload = _build_auto_payload(defaults)
    for path in candidate_paths:
        if _write_identity_payload(path, auto_payload):
            return _public_identity(path, auto_payload, "device_identity_file_created")

    return _public_identity(None, auto_payload, "device_identity_runtime_fallback")
