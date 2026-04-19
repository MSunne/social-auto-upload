#!/usr/bin/env python3

from __future__ import annotations

import argparse
import fnmatch
import os
import shlex
import subprocess
import sys
import tarfile
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path


ROOT_DIR = Path(__file__).resolve().parents[1]
SSH_KEY_PATH = Path.home() / ".ssh" / "omnidrive_live_sync"
REMOTE_HOST = "root@43.98.251.225"
TMP_API_BINARY = Path("/tmp/omnidrive-api-live-sync")
TMP_BOOTSTRAP_BINARY = Path("/tmp/omnidrive-bootstrap-db-live-sync")
LOCAL_DEMO_SEED_SCRIPT = ROOT_DIR / "scripts" / "seed_omnidrive_mock_data.py"
REMOTE_DEMO_SEED_SCRIPT = Path("/tmp/seed_omnidrive_mock_data.py")
REMOTE_DEMO_SEED_ROOT = Path("/www/wwwroot/OmniDriveCloud/data/mock-seed")
POLL_INTERVAL_SECONDS = float(os.environ.get("OMNIDRIVE_LIVE_SYNC_POLL_INTERVAL_SECONDS", "2"))
DEBOUNCE_SECONDS = float(os.environ.get("OMNIDRIVE_LIVE_SYNC_DEBOUNCE_SECONDS", "2"))


@dataclass(frozen=True)
class Target:
    name: str
    local_dir: Path
    remote_dir: str
    rsync_excludes: tuple[str, ...]
    start_port: int | None = None


TARGETS: dict[str, Target] = {
    "omnidrive_cloud": Target(
        name="omnidrive_cloud",
        local_dir=ROOT_DIR / "omnidrive_cloud",
        remote_dir="/www/wwwroot/OmniDriveCloud",
        rsync_excludes=(
            ".git",
            ".env",
            ".env.*",
            "bin",
            "data",
            "data-smoke",
            "*.log",
        ),
    ),
    "omnidrive_frontend": Target(
        name="omnidrive_frontend",
        local_dir=ROOT_DIR / "omnidrive_frontend",
        remote_dir="/www/wwwroot/aitoplus.com",
        start_port=3000,
        rsync_excludes=(
            ".git",
            ".env",
            ".env.*",
            ".next",
            "node_modules",
            "*.log",
        ),
    ),
    "OmniDriveAdmin": Target(
        name="OmniDriveAdmin",
        local_dir=ROOT_DIR / "OmniDriveAdmin",
        remote_dir="/www/wwwroot/omnidrive_admin",
        start_port=3001,
        rsync_excludes=(
            ".git",
            ".env",
            ".env.*",
            ".next",
            "node_modules",
            "*.log",
        ),
    ),
}


def log(message: str) -> None:
    timestamp = time.strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{timestamp}] {message}", flush=True)


def require_command(name: str) -> None:
    if subprocess.run(["/usr/bin/env", "bash", "-lc", f"command -v {name} >/dev/null 2>&1"]).returncode != 0:
        raise RuntimeError(f"missing required command: {name}")


def run_command(cmd: list[str], *, cwd: Path | None = None) -> None:
    log(f"run: {' '.join(cmd)}")
    subprocess.run(cmd, cwd=cwd, check=True)


def run_shell(script: str, *, cwd: Path | None = None) -> None:
    log(f"run shell: {script}")
    subprocess.run(["/bin/zsh", "-lc", script], cwd=cwd, check=True)


def remote_shell(script: str) -> None:
    run_command(
        [
            "ssh",
            "-i",
            str(SSH_KEY_PATH),
            "-o",
            "BatchMode=yes",
            REMOTE_HOST,
            script,
        ]
    )


def should_exclude(target: Target, rel_path: Path, is_dir: bool) -> bool:
    rel_text = rel_path.as_posix()
    for pattern in target.rsync_excludes:
        if fnmatch.fnmatch(rel_text, pattern):
            return True
        if is_dir and fnmatch.fnmatch(rel_path.name, pattern):
            return True
        if not is_dir and fnmatch.fnmatch(rel_path.name, pattern):
            return True
    return False


def snapshot_target(target: Target) -> dict[str, tuple[int, int]]:
    snapshot: dict[str, tuple[int, int]] = {}
    for current_root, dirs, files in os.walk(target.local_dir):
        current_root_path = Path(current_root)
        rel_root = current_root_path.relative_to(target.local_dir)
        dirs[:] = [
            item
            for item in dirs
            if not should_exclude(
                target,
                (rel_root / item) if rel_root != Path(".") else Path(item),
                True,
            )
        ]
        for filename in files:
            rel_path = (rel_root / filename) if rel_root != Path(".") else Path(filename)
            if should_exclude(target, rel_path, False):
                continue
            file_path = current_root_path / filename
            stat = file_path.stat()
            snapshot[rel_path.as_posix()] = (stat.st_mtime_ns, stat.st_size)
    return snapshot


def capture_state() -> dict[str, dict[str, tuple[int, int]]]:
    return {name: snapshot_target(target) for name, target in TARGETS.items()}


def sync_target_files(target: Target, changed_files: set[str], removed_files: set[str]) -> None:
    remote_dir_quoted = shlex.quote(target.remote_dir)
    if removed_files:
        delete_script = "set -e; " + " ".join(
            f"rm -f {shlex.quote(f'{target.remote_dir}/{path}')};" for path in sorted(removed_files)
        )
        remote_shell(delete_script)

    if not changed_files:
        return

    with tempfile.NamedTemporaryFile(prefix=f"{target.name}-", suffix=".tar.gz", delete=False) as tmp_file:
        archive_path = Path(tmp_file.name)

    try:
        with tarfile.open(archive_path, "w:gz") as archive:
            for rel_path in sorted(changed_files):
                archive.add(target.local_dir / rel_path, arcname=rel_path)

        remote_archive = f"/tmp/{archive_path.name}"
        run_command(
            [
                "scp",
                "-i",
                str(SSH_KEY_PATH),
                "-o",
                "BatchMode=yes",
                str(archive_path),
                f"{REMOTE_HOST}:{remote_archive}",
            ]
        )
        remote_shell(
            "set -e; "
            f"mkdir -p {remote_dir_quoted}; "
            f"tar -xzf {shlex.quote(remote_archive)} -C {remote_dir_quoted}; "
            f"rm -f {shlex.quote(remote_archive)}"
        )
    finally:
        archive_path.unlink(missing_ok=True)


def build_cloud_binaries() -> None:
    run_shell(
        "cd omnidrive_cloud && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "
        f"go build -o {TMP_API_BINARY} ./cmd/omnidrive-api",
        cwd=ROOT_DIR,
    )
    run_shell(
        "cd omnidrive_cloud && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 "
        f"go build -o {TMP_BOOTSTRAP_BINARY} ./cmd/omnidrive-bootstrap-db",
        cwd=ROOT_DIR,
    )


def upload_cloud_binaries() -> None:
    run_command(
        [
            "scp",
            "-i",
            str(SSH_KEY_PATH),
            "-o",
            "BatchMode=yes",
            str(TMP_API_BINARY),
            f"{REMOTE_HOST}:{TMP_API_BINARY}",
        ]
    )
    run_command(
        [
            "scp",
            "-i",
            str(SSH_KEY_PATH),
            "-o",
            "BatchMode=yes",
            str(TMP_BOOTSTRAP_BINARY),
            f"{REMOTE_HOST}:{TMP_BOOTSTRAP_BINARY}",
        ]
    )


def restart_cloud_api() -> None:
    remote_shell(
        "set -e; "
        f"install -m 0755 {TMP_BOOTSTRAP_BINARY} /www/wwwroot/OmniDriveCloud/bin/omnidrive-bootstrap-db; "
        f"install -m 0755 {TMP_API_BINARY} /www/wwwroot/OmniDriveCloud/bin/omnidrive-api; "
        "su -s /bin/bash www -c 'cd /www/wwwroot/OmniDriveCloud && ./bin/omnidrive-bootstrap-db'; "
        "pkill -x omnidrive-api || true; "
        "sleep 1; "
        "setsid su -s /bin/bash www -c 'cd /www/wwwroot/OmniDriveCloud && "
        "exec ./bin/omnidrive-api >> /www/wwwroot/OmniDriveCloud/omnidrive-api.log 2>&1' "
        ">/dev/null 2>&1 </dev/null & "
        "echo $! > /www/wwwroot/OmniDriveCloud/omnidrive-api.pid; "
        "sleep 3; "
        "curl -sf http://127.0.0.1:8410/health >/dev/null; "
        "curl -sf http://127.0.0.1:8410/ready >/dev/null"
    )


def build_remote_next_app(target: Target) -> None:
    remote_shell(f"set -e; cd {target.remote_dir} && npm run build")


def restart_remote_next_app(target: Target) -> None:
    if target.start_port is None:
        raise RuntimeError(f"target {target.name} does not define a start port")

    pid_file = f"/tmp/{target.name}.next.pid"
    remote_shell(
        "set -e; "
        f"cd {target.remote_dir}; "
        f"if [ -f {pid_file} ]; then "
        f"  kill $(cat {pid_file}) >/dev/null 2>&1 || true; "
        "fi; "
        "for _ in $(seq 1 20); do "
        f"  if [ -f {pid_file} ] && kill -0 $(cat {pid_file}) >/dev/null 2>&1; then "
        "    sleep 1; "
        "  else "
        "    break; "
        "  fi; "
        "done; "
        f"if command -v fuser >/dev/null 2>&1; then fuser -k {target.start_port}/tcp >/dev/null 2>&1 || true; fi; "
        "sleep 1; "
        f"rm -f {pid_file}; "
        f"setsid su -s /bin/bash www -c 'cd {target.remote_dir} && : > start.log && exec npm run start >start.log 2>&1' "
        f">/dev/null 2>&1 </dev/null & echo $! > {pid_file}; "
        f"for _ in $(seq 1 60); do if curl -sf http://127.0.0.1:{target.start_port}/ >/dev/null; then exit 0; fi; sleep 1; done; "
        "echo 'failed to restart next app' >&2; "
        "tail -n 80 start.log >&2; "
        "exit 1"
    )


def verify_remote_log_is_clean() -> None:
    remote_shell("grep -n 'conn busy' /www/wwwroot/OmniDriveCloud/omnidrive-api.log || true")


def upload_demo_seed_script() -> None:
    run_command(
        [
            "scp",
            "-i",
            str(SSH_KEY_PATH),
            "-o",
            "BatchMode=yes",
            str(LOCAL_DEMO_SEED_SCRIPT),
            f"{REMOTE_HOST}:{REMOTE_DEMO_SEED_SCRIPT}",
        ]
    )


def seed_remote_demo_data() -> None:
    upload_demo_seed_script()
    remote_shell(
        "set -e; "
        f"mkdir -p {shlex.quote(str(REMOTE_DEMO_SEED_ROOT))}; "
        "python3 "
        f"{shlex.quote(str(REMOTE_DEMO_SEED_SCRIPT))} "
        "--base-url http://127.0.0.1:8410 "
        f"--seed-root {shlex.quote(str(REMOTE_DEMO_SEED_ROOT))}"
    )


def sync_targets(changed_files: dict[str, set[str]], removed_files: dict[str, set[str]], *, seed_demo_data: bool = False) -> None:
    target_names = {name for name in TARGETS if changed_files[name] or removed_files[name]}
    if not target_names:
        return

    if "omnidrive_cloud" in target_names:
        log("syncing omnidrive_cloud")
        sync_target_files(TARGETS["omnidrive_cloud"], changed_files["omnidrive_cloud"], removed_files["omnidrive_cloud"])
        build_cloud_binaries()
        upload_cloud_binaries()
        restart_cloud_api()
        verify_remote_log_is_clean()
        if seed_demo_data:
            seed_remote_demo_data()
        log("synced omnidrive_cloud")

    frontend_targets = [name for name in ("omnidrive_frontend", "OmniDriveAdmin") if name in target_names]
    if frontend_targets:
        for name in frontend_targets:
            log(f"syncing {name}")
            sync_target_files(TARGETS[name], changed_files[name], removed_files[name])
            build_remote_next_app(TARGETS[name])
            restart_remote_next_app(TARGETS[name])
        log(f"synced {', '.join(frontend_targets)}")


def ensure_prerequisites() -> None:
    if not SSH_KEY_PATH.exists():
        raise RuntimeError(f"ssh key not found: {SSH_KEY_PATH}")
    for command in ("scp", "ssh", "go", "python3", "tar"):
        require_command(command)
    if not LOCAL_DEMO_SEED_SCRIPT.exists():
        raise RuntimeError(f"demo seed script not found: {LOCAL_DEMO_SEED_SCRIPT}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Continuously sync local OmniDrive code to cloud.")
    parser.add_argument("--once", action="store_true", help="sync all cloud targets once and exit")
    parser.add_argument("--skip-initial-sync", action="store_true", help="start watching from the current local snapshot")
    parser.add_argument("--seed-demo-data", action="store_true", help="run the OmniDrive demo seed on the 43 server after cloud sync")
    args = parser.parse_args()

    ensure_prerequisites()

    log("starting cloud live sync")
    state = capture_state()
    if not args.skip_initial_sync:
        initial_changes = {name: set(state[name].keys()) for name in TARGETS}
        initial_removals = {name: set() for name in TARGETS}
        sync_targets(initial_changes, initial_removals, seed_demo_data=args.seed_demo_data)
        state = capture_state()
    if args.once:
        log("cloud live sync completed once")
        return 0

    pending_changes = {name: set() for name in TARGETS}
    pending_removals = {name: set() for name in TARGETS}
    last_change_monotonic = 0.0

    while True:
        time.sleep(POLL_INTERVAL_SECONDS)
        current_state = capture_state()
        changed_targets = []
        for name in TARGETS:
            if current_state[name] == state[name]:
                continue
            changed_targets.append(name)
            previous = state[name]
            current = current_state[name]
            added_or_modified = {path for path, metadata in current.items() if previous.get(path) != metadata}
            removed = set(previous) - set(current)
            pending_changes[name].update(added_or_modified)
            pending_removals[name].update(removed)
            pending_removals[name].difference_update(added_or_modified)
            pending_changes[name].difference_update(removed)

        if changed_targets:
            last_change_monotonic = time.monotonic()
            state = current_state
            log(f"detected changes in: {', '.join(sorted(changed_targets))}")
            continue
        if any(pending_changes[name] or pending_removals[name] for name in TARGETS) and time.monotonic() - last_change_monotonic >= DEBOUNCE_SECONDS:
            changes_to_sync = {name: set(pending_changes[name]) for name in TARGETS}
            removals_to_sync = {name: set(pending_removals[name]) for name in TARGETS}
            pending_changes = {name: set() for name in TARGETS}
            pending_removals = {name: set() for name in TARGETS}
            try:
                sync_targets(changes_to_sync, removals_to_sync, seed_demo_data=args.seed_demo_data)
            except subprocess.CalledProcessError as exc:
                log(f"sync failed with exit code {exc.returncode}; will retry")
                for name in TARGETS:
                    pending_changes[name].update(changes_to_sync[name])
                    pending_removals[name].update(removals_to_sync[name])
                last_change_monotonic = time.monotonic()
            except Exception as exc:  # noqa: BLE001
                log(f"sync failed with error: {exc}; will retry")
                for name in TARGETS:
                    pending_changes[name].update(changes_to_sync[name])
                    pending_removals[name].update(removals_to_sync[name])
                last_change_monotonic = time.monotonic()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        log("cloud live sync stopped")
        raise SystemExit(0)
