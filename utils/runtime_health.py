import os
import tempfile
from pathlib import Path

from utils.browser_hook import describe_browser_runtime


def _check_directory(path, *, create=False):
    resolved = Path(path).expanduser().resolve()
    issues = []

    if create:
        try:
            resolved.mkdir(parents=True, exist_ok=True)
        except OSError as exc:
            issues.append(f"create failed: {exc}")

    exists = resolved.exists()
    is_directory = resolved.is_dir()
    readable = exists and os.access(resolved, os.R_OK)
    writable = exists and is_directory and os.access(resolved, os.W_OK | os.X_OK)
    write_probe = False

    if exists and not is_directory:
        issues.append("path exists but is not a directory")
    elif not exists:
        issues.append("path does not exist")
    elif not readable:
        issues.append("path is not readable")

    if exists and is_directory:
        if not writable:
            issues.append("path is not writable")
        else:
            try:
                with tempfile.NamedTemporaryFile(
                    prefix=".omnibull_probe_",
                    dir=resolved,
                    delete=True,
                ) as handle:
                    handle.write(b"ok")
                    handle.flush()
                write_probe = True
            except OSError as exc:
                issues.append(f"write probe failed: {exc}")

    return {
        "path": str(resolved),
        "exists": exists,
        "isDirectory": is_directory,
        "readable": readable,
        "writable": writable,
        "writeProbe": write_probe,
        "ok": not issues,
        "issues": issues,
    }


def build_runtime_health(*, base_dir, material_roots, device_identity_path=None, generated_root_path=None, headless=False):
    base_path = Path(base_dir).resolve()
    path_checks = {
        "db": _check_directory(base_path / "db", create=True),
        "videoFile": _check_directory(base_path / "videoFile", create=True),
        "cookies": _check_directory(base_path / "cookies", create=True),
        "cookiesFile": _check_directory(base_path / "cookiesFile", create=True),
    }

    if generated_root_path:
        path_checks["generatedRoot"] = _check_directory(generated_root_path, create=True)

    if device_identity_path:
        path_checks["deviceIdentityParent"] = _check_directory(
            Path(device_identity_path).expanduser().resolve().parent,
            create=True,
        )

    material_root_checks = {}
    for root_name, root_path in material_roots.items():
        material_root_checks[root_name] = _check_directory(root_path, create=True)

    browser = describe_browser_runtime(headless=headless)
    path_issues = [
        {"name": name, "issues": item["issues"], "path": item["path"]}
        for name, item in path_checks.items()
        if not item["ok"]
    ]
    material_root_issues = [
        {"name": name, "issues": item["issues"], "path": item["path"]}
        for name, item in material_root_checks.items()
        if not item["ok"]
    ]

    issues = []
    if browser["issues"]:
        issues.append({"name": "browser", "issues": browser["issues"]})
    issues.extend(path_issues)
    issues.extend(material_root_issues)

    return {
        "ok": browser["available"] and not path_issues and not material_root_issues,
        "browser": browser,
        "paths": path_checks,
        "materialRoots": material_root_checks,
        "issues": issues,
    }


def log_runtime_health(logger, payload):
    browser = payload.get("browser", {})
    logger.info(
        "OmniBull runtime health browser_available={} browser_source={} issue_count={}",
        browser.get("available"),
        browser.get("source"),
        len(payload.get("issues") or []),
    )

    for issue in payload.get("issues") or []:
        logger.warning(
            "OmniBull runtime health issue name={} path={} issues={}",
            issue.get("name"),
            issue.get("path"),
            ", ".join(issue.get("issues") or []),
        )
