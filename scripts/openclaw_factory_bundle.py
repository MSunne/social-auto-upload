#!/usr/bin/env python3

from __future__ import annotations

import argparse
import copy
import json
import shutil
from pathlib import Path
from typing import Any


PLUGIN_ALLOW_ORDER = [
    "feishu",
    "wecom-openclaw-plugin",
    "omnibull",
    "omnidrive",
    "openclaw-weixin",
]
LOCAL_EXTENSION_IDS = [
    "omnibull",
    "omnidrive",
    "feishu",
    "openclaw-weixin",
    "wecom-openclaw-plugin",
]
LOCAL_STATE_DIRS = ["feishu", "openclaw-weixin", "wecom", "wecomConfig"]
LOCAL_CREDENTIAL_PATTERNS = [
    "feishu*.json",
    "wecom*.json",
    "wechat*.json",
    "weixin*.json",
]


def _deep_copy_json(value: Any) -> Any:
    return json.loads(json.dumps(value))


def _load_json(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def _write_json(path: Path, payload: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )


def _copy_path(source: Path, target: Path) -> None:
    if not source.exists():
        return
    if target.exists():
        if target.is_dir() and not target.is_symlink():
            shutil.rmtree(target)
        else:
            target.unlink()
    target.parent.mkdir(parents=True, exist_ok=True)
    if source.is_dir():
        shutil.copytree(source, target)
        return
    shutil.copy2(source, target)


def build_deploy_manifest(
    source_config: dict[str, Any],
    app_root: str,
    omnidrive_base_url: str,
) -> dict[str, Any]:
    plugin_entries = (((source_config.get("plugins") or {}).get("entries")) or {})
    plugin_installs = (((source_config.get("plugins") or {}).get("installs")) or {})
    source_skill_entries = (((source_config.get("skills") or {}).get("entries")) or {})

    skills_entries = {
        key: {"enabled": True}
        for key, value in source_skill_entries.items()
        if isinstance(value, dict) and value.get("enabled") is True
    }

    install_specs = {}
    for plugin_id in ("feishu", "wecom-openclaw-plugin"):
        install_payload = plugin_installs.get(plugin_id) or {}
        resolved_spec = str(
            install_payload.get("resolvedSpec") or install_payload.get("spec") or ""
        ).strip()
        if resolved_spec:
            install_specs[plugin_id] = resolved_spec

    manifest = {
        "plugins": {
            "allow": PLUGIN_ALLOW_ORDER[:],
            "load": {"paths": []},
            "entries": {
                "feishu": {"enabled": bool((plugin_entries.get("feishu") or {}).get("enabled", True))},
                "wecom-openclaw-plugin": {
                    "enabled": bool(
                        (plugin_entries.get("wecom-openclaw-plugin") or {}).get(
                            "enabled", False
                        )
                    )
                },
                "omnibull": {
                    "enabled": True,
                    "config": {
                        "baseUrl": "http://127.0.0.1:5409",
                        "apiKey": "",
                        "timeoutMs": 15000,
                    },
                },
                "omnidrive": {
                    "enabled": bool((plugin_entries.get("omnidrive") or {}).get("enabled", True)),
                    "config": {
                        "baseUrl": omnidrive_base_url,
                        "localOmniBullBaseUrl": "http://127.0.0.1:5409",
                        "timeoutMs": 45000,
                    },
                },
                "openclaw-weixin": {
                    "enabled": bool(
                        (plugin_entries.get("openclaw-weixin") or {}).get("enabled", True)
                    )
                },
            },
            "install_specs": install_specs,
        },
        "skills": {"entries": skills_entries},
    }
    return manifest


def merge_remote_config(
    existing: dict[str, Any],
    manifest: dict[str, Any],
    *,
    gateway_bind: str = "loopback",
    gateway_port: int = 18790,
) -> dict[str, Any]:
    merged = _deep_copy_json(existing or {})
    gateway = copy.deepcopy(merged.get("gateway") or {})
    gateway["mode"] = "local"
    gateway["bind"] = gateway_bind
    gateway["port"] = gateway_port
    gateway["auth"] = {"mode": "none"}
    control_ui = copy.deepcopy(gateway.get("controlUi") or {})
    control_ui["allowedOrigins"] = ["http://192.168.1.24:18789"]
    control_ui["dangerouslyDisableDeviceAuth"] = True
    gateway["controlUi"] = control_ui
    merged["gateway"] = gateway

    plugins = copy.deepcopy(merged.get("plugins") or {})
    plugins["allow"] = _deep_copy_json(manifest["plugins"]["allow"])
    plugins["load"] = _deep_copy_json(manifest["plugins"]["load"])
    plugin_entries = copy.deepcopy(plugins.get("entries") or {})
    plugin_entries.pop("qwen-portal-auth", None)
    for plugin_id, payload in manifest["plugins"]["entries"].items():
        plugin_entries[plugin_id] = _deep_copy_json(payload)
    plugins["entries"] = plugin_entries
    merged["plugins"] = plugins

    skills = copy.deepcopy(merged.get("skills") or {})
    skill_entries = copy.deepcopy(skills.get("entries") or {})
    for skill_id, payload in manifest["skills"]["entries"].items():
        skill_entries[skill_id] = _deep_copy_json(payload)
    skills["entries"] = skill_entries
    merged["skills"] = skills
    return merged


def export_bundle(
    source_home: Path,
    output_dir: Path,
    app_root: str,
    omnidrive_base_url: str,
) -> dict[str, Any]:
    source_home = source_home.expanduser().resolve()
    output_dir = output_dir.expanduser().resolve()
    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    source_config = _load_json(source_home / "openclaw.json")
    manifest = build_deploy_manifest(
        source_config=source_config,
        app_root=app_root,
        omnidrive_base_url=omnidrive_base_url,
    )

    extensions_output = output_dir / "extensions"
    _copy_path(
        Path(__file__).resolve().parents[1] / "openclaw_extensions" / "omnibull",
        extensions_output / "omnibull",
    )
    for plugin_id in LOCAL_EXTENSION_IDS:
        _copy_path(source_home / "extensions" / plugin_id, extensions_output / plugin_id)

    state_output = output_dir / "state"
    for state_dir in LOCAL_STATE_DIRS:
        _copy_path(source_home / state_dir, state_output / state_dir)

    credential_dir = source_home / "credentials"
    if credential_dir.exists():
        target_credential_dir = state_output / "credentials"
        for pattern in LOCAL_CREDENTIAL_PATTERNS:
            for source_path in credential_dir.glob(pattern):
                _copy_path(source_path, target_credential_dir / source_path.name)

    _write_json(output_dir / "manifest.json", manifest)
    return manifest


def apply_bundle(
    bundle_dir: Path,
    target_home: Path,
    *,
    gateway_bind: str = "loopback",
    gateway_port: int = 18790,
) -> dict[str, Any]:
    bundle_dir = bundle_dir.expanduser().resolve()
    target_home = target_home.expanduser().resolve()
    manifest = _load_json(bundle_dir / "manifest.json")
    target_openclaw_dir = target_home / ".openclaw"
    target_openclaw_dir.mkdir(parents=True, exist_ok=True)

    state_dir = bundle_dir / "state"
    for state_name in LOCAL_STATE_DIRS:
        _copy_path(state_dir / state_name, target_openclaw_dir / state_name)

    credential_source_dir = state_dir / "credentials"
    if credential_source_dir.exists():
        target_credential_dir = target_openclaw_dir / "credentials"
        target_credential_dir.mkdir(parents=True, exist_ok=True)
        for child in credential_source_dir.iterdir():
            _copy_path(child, target_credential_dir / child.name)

    existing_config = _load_json(target_openclaw_dir / "openclaw.json")
    merged = merge_remote_config(
        existing_config,
        manifest,
        gateway_bind=gateway_bind,
        gateway_port=gateway_port,
    )
    _write_json(target_openclaw_dir / "openclaw.json", merged)
    return merged


def _cmd_export(args: argparse.Namespace) -> int:
    manifest = export_bundle(
        source_home=Path(args.source_home),
        output_dir=Path(args.output_dir),
        app_root=args.app_root,
        omnidrive_base_url=args.omnidrive_base_url,
    )
    print(json.dumps(manifest, ensure_ascii=False, indent=2))
    return 0


def _cmd_apply(args: argparse.Namespace) -> int:
    config = apply_bundle(
        bundle_dir=Path(args.bundle_dir),
        target_home=Path(args.target_home),
        gateway_bind=args.gateway_bind,
        gateway_port=args.gateway_port,
    )
    print(json.dumps(config, ensure_ascii=False, indent=2))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Prepare and apply OpenClaw deployment bundles for factory devices."
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    export_parser = subparsers.add_parser("export", help="Export a deploy bundle from a local ~/.openclaw")
    export_parser.add_argument("--source-home", required=True)
    export_parser.add_argument("--output-dir", required=True)
    export_parser.add_argument("--app-root", required=True)
    export_parser.add_argument("--omnidrive-base-url", required=True)
    export_parser.set_defaults(func=_cmd_export)

    apply_parser = subparsers.add_parser("apply", help="Apply a deploy bundle into a target ~/.openclaw")
    apply_parser.add_argument("--bundle-dir", required=True)
    apply_parser.add_argument("--target-home", required=True)
    apply_parser.add_argument("--gateway-bind", default="loopback")
    apply_parser.add_argument("--gateway-port", type=int, default=18790)
    apply_parser.set_defaults(func=_cmd_apply)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    raise SystemExit(main())
