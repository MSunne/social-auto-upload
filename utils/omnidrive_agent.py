import base64
import json
import mimetypes
import shutil
import socket
import sqlite3
import threading
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from queue import Empty, Queue

import requests

from conf import BASE_DIR
from utils.device_meta import get_local_ip
from utils.log import agent_logger, log_throttled
from utils.materials import list_material_directory, list_material_roots, read_material_file


LOGIN_PLATFORM_CONFIG = {
    "xiaohongshu": {
        "type": 1,
        "label": "小红书",
        "aliases": ("xiaohongshu", "小红书"),
    },
    "wechat_channel": {
        "type": 2,
        "label": "视频号",
        "aliases": ("wechat_channel", "视频号", "wechat"),
    },
    "douyin": {
        "type": 3,
        "label": "抖音",
        "aliases": ("douyin", "抖音"),
    },
    "kuaishou": {
        "type": 4,
        "label": "快手",
        "aliases": ("kuaishou", "快手"),
    },
}

PLATFORM_TYPE_BY_NAME = {
    config["label"]: config["type"]
    for config in LOGIN_PLATFORM_CONFIG.values()
}

PLATFORM_NAME_BY_TYPE = {
    config["type"]: config["label"]
    for config in LOGIN_PLATFORM_CONFIG.values()
}

LOGIN_PLATFORM_ALIAS_MAP = {}
for platform_slug, config in LOGIN_PLATFORM_CONFIG.items():
    normalized_aliases = {
        str(alias or "").strip().lower()
        for alias in config.get("aliases") or ()
        if str(alias or "").strip()
    }
    normalized_aliases.add(platform_slug)
    normalized_aliases.add(str(config.get("label") or "").strip().lower())
    for alias in normalized_aliases:
        LOGIN_PLATFORM_ALIAS_MAP[alias] = {
            "slug": platform_slug,
            "type": int(config["type"]),
            "label": str(config["label"]).strip(),
        }

SYNCABLE_LOCAL_SOURCES = ("local_api", "openclaw_skill", "omnidrive_agent", "omnidrive_ai")
SYNCABLE_LOCAL_AI_SOURCES = ("local_ui", "openclaw_skill")
FINAL_LOCAL_STATUSES = {"success", "failed", "needs_verify", "cancelled"}
FINAL_LOGIN_STATUSES = {"success", "failed", "cancelled"}
LOGIN_PENDING_STALE_SECONDS = 90
LOGIN_RUNNING_STALE_SECONDS = 60
LOGIN_VERIFICATION_STALE_SECONDS = 180


class OmniDriveBridge:
    def __init__(
        self,
        db_path,
        cloud_base_url,
        agent_key,
        run_login_fn,
        publish_task_manager,
        ai_task_manager,
        material_roots,
        device_name,
        device_code,
        generated_root_name,
        generated_root_path,
        poll_interval=5,
        heartbeat_interval=30,
        account_sync_interval=60,
        material_sync_interval=300,
        skill_sync_interval=120,
        publish_sync_interval=5,
        max_material_files=1000,
        material_preview_bytes=65536,
        http_timeout=15,
    ):
        self.db_path = Path(db_path)
        self.cloud_base_url = str(cloud_base_url or "").rstrip("/")
        self.agent_key = str(agent_key or "").strip()
        self.run_login_fn = run_login_fn
        self.publish_task_manager = publish_task_manager
        self.ai_task_manager = ai_task_manager
        self.material_roots = material_roots or {}
        self.device_name = str(device_name or socket.gethostname()).strip() or socket.gethostname()
        self.device_code = str(device_code or "").strip()
        self.generated_root_name = str(generated_root_name or "omnidriveGenerated").strip() or "omnidriveGenerated"
        self.generated_root_path = Path(generated_root_path).resolve()
        self.generated_root_path.mkdir(parents=True, exist_ok=True)
        self.poll_interval = max(2, int(poll_interval))
        self.heartbeat_interval = max(10, int(heartbeat_interval))
        self.account_sync_interval = max(10, int(account_sync_interval))
        self.material_sync_interval = max(60, int(material_sync_interval))
        self.skill_sync_interval = max(30, int(skill_sync_interval))
        self.publish_sync_interval = max(2, int(publish_sync_interval))
        self.max_material_files = max(50, int(max_material_files))
        self.material_preview_bytes = max(1024, int(material_preview_bytes))
        self.http_timeout = max(5, int(http_timeout))

        self._thread = None
        self._thread_lock = threading.Lock()
        self._stop_event = threading.Event()
        self._session = requests.Session()
        self._state_lock = threading.Lock()
        self._login_worker_lock = threading.Lock()
        self._active_login_worker = None
        self._agent_started_at_epoch = time.time()
        self._login_startup_cleanup_done = False
        self._state = {
            "running": False,
            "deviceName": self.device_name,
            "deviceCode": self.device_code,
            "cloudUrl": self.cloud_base_url,
            "lastHeartbeatAt": None,
            "lastAccountSyncAt": None,
            "lastMaterialSyncAt": None,
            "lastSkillSyncAt": None,
            "lastPublishPollAt": None,
            "lastPublishSyncAt": None,
            "lastLeaseRenewAt": None,
            "lastAISyncAt": None,
            "lastAIPollAt": None,
            "lastLoginPollAt": None,
            "lastLoginEventAt": None,
            "lastError": None,
            "localIp": get_local_ip(),
            "syncedRoots": 0,
            "syncedFiles": 0,
            "syncedSkills": 0,
            "retiredSkillAcks": 0,
            "mirroredAccounts": 0,
            "mirroredTasks": 0,
            "importedCloudTasks": 0,
            "mirroredAITasks": 0,
            "importedAIResults": 0,
            "activeLoginSessionCount": 0,
            "activeLoginSessions": [],
        }

        self._workspace_dir = Path(BASE_DIR / "omnidriveSync")
        self._skill_cache_dir = self._workspace_dir / "skills"
        self._skill_cache_dir.mkdir(parents=True, exist_ok=True)
        self._init_db()

    @property
    def enabled(self):
        return bool(self.cloud_base_url and self.agent_key and self.device_code)

    def start(self):
        if not self.enabled:
            return
        with self._thread_lock:
            if self._thread and self._thread.is_alive():
                return
            self._stop_event.clear()
            self._init_db()
            self._agent_started_at_epoch = time.time()
            self._login_startup_cleanup_done = False
            self._thread = threading.Thread(target=self._loop, daemon=True)
            self._thread.start()
            agent_logger.info(
                "omnidrive bridge started device_code={} device_name={} poll_interval={} heartbeat_interval={} account_sync_interval={} material_sync_interval={} skill_sync_interval={} publish_sync_interval={}",
                self.device_code,
                self.device_name,
                self.poll_interval,
                self.heartbeat_interval,
                self.account_sync_interval,
                self.material_sync_interval,
                self.skill_sync_interval,
                self.publish_sync_interval,
            )

    def status(self):
        with self._state_lock:
            snapshot = dict(self._state)
        snapshot.update(self._build_status_snapshot())
        return snapshot

    def list_cached_skills(self, include_assets=False):
        if not self._skill_cache_dir.exists():
            return []

        items = []
        for manifest_path in sorted(self._skill_cache_dir.glob("*/manifest.json")):
            manifest = self._load_cached_skill_manifest(manifest_path)
            if not manifest:
                continue
            items.append(
                self._build_cached_skill_snapshot(
                    manifest,
                    manifest_path,
                    include_assets=include_assets,
                )
            )
        return items

    def get_cached_skill(self, skill_id):
        skill_id = str(skill_id or "").strip()
        if not skill_id:
            return None
        manifest_path = self._skill_cache_dir / skill_id / "manifest.json"
        manifest = self._load_cached_skill_manifest(manifest_path)
        if not manifest:
            return None
        return self._build_cached_skill_snapshot(manifest, manifest_path, include_assets=True)

    def _update_state(self, **kwargs):
        with self._state_lock:
            self._state.update(kwargs)

    def _loop(self):
        self._update_state(running=True, lastError=None)
        last_heartbeat = 0.0
        last_accounts = 0.0
        last_materials = 0.0
        last_skills = 0.0
        last_publish_sync = 0.0
        last_publish_poll = 0.0
        last_lease_renew = 0.0
        last_ai_sync = 0.0
        last_ai_poll = 0.0
        last_login_poll = 0.0

        while not self._stop_event.is_set():
            try:
                now = time.monotonic()
                if now - last_heartbeat >= self.heartbeat_interval:
                    self._heartbeat()
                    last_heartbeat = now
                if now - last_accounts >= self.account_sync_interval:
                    self._sync_accounts()
                    last_accounts = now
                if now - last_materials >= self.material_sync_interval:
                    self._sync_materials()
                    last_materials = now
                if now - last_skills >= self.skill_sync_interval:
                    self._sync_skills()
                    last_skills = now
                if now - last_publish_sync >= self.publish_sync_interval:
                    self._sync_local_publish_tasks()
                    last_publish_sync = now
                if now - last_lease_renew >= self.publish_sync_interval:
                    self._renew_active_leases()
                    last_lease_renew = now
                if now - last_publish_poll >= self.poll_interval:
                    self._import_remote_publish_tasks()
                    last_publish_poll = now
                if now - last_ai_sync >= self.publish_sync_interval:
                    self._sync_local_ai_tasks()
                    self._sync_local_ai_publish_state()
                    last_ai_sync = now
                if now - last_ai_poll >= self.poll_interval:
                    self._import_remote_ai_jobs()
                    last_ai_poll = now
                self._sync_active_login_session()
                if now - last_login_poll >= self.poll_interval:
                    self._poll_remote_login_tasks()
                    last_login_poll = now
            except Exception as exc:
                self._update_state(lastError=str(exc))
                log_throttled(
                    agent_logger,
                    "ERROR",
                    f"omnidrive_agent.loop_error:{self.device_code}",
                    30,
                    "omnidrive bridge loop failed device_code={} error={}",
                    self.device_code,
                    exc,
                )

            self._stop_event.wait(1)

        self._update_state(running=False)

    def _headers(self):
        return {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "X-Agent-Key": self.agent_key,
        }

    def _request(self, method, path, *, params=None, payload=None):
        response = self._session.request(
            method=method,
            url=f"{self.cloud_base_url}{path}",
            headers=self._headers(),
            params=params,
            json=payload,
            timeout=self.http_timeout,
        )
        response.raise_for_status()
        if not response.content:
            return None
        return response.json()

    def _heartbeat(self):
        data = self._request(
            "POST",
            "/api/v1/agent/heartbeat",
            payload={
                "deviceCode": self.device_code,
                "deviceName": self.device_name,
                "agentKey": self.agent_key,
                "localIp": get_local_ip(),
                "runtimePayload": self._build_runtime_payload(),
            },
        )
        self._update_state(
            lastHeartbeatAt=self._now_string(),
            lastError=None,
            localIp=get_local_ip(),
        )
        log_throttled(
            agent_logger,
            "DEBUG",
            f"omnidrive_agent.heartbeat:{self.device_code}",
            max(self.heartbeat_interval * 4, 120),
            "omnidrive bridge heartbeat ok device_code={} local_ip={}",
            self.device_code,
            get_local_ip(),
        )
        return data

    def _sync_accounts(self):
        rows = self._load_local_accounts()
        mirrored = 0
        for row in rows:
            platform_name = PLATFORM_NAME_BY_TYPE.get(int(row["type"]))
            if not platform_name:
                continue
            status = "active" if int(row["status"] or 0) == 1 else "inactive"
            payload = {
                "deviceCode": self.device_code,
                "platform": platform_name,
                "accountName": row["userName"],
                "status": status,
                "lastMessage": None if status == "active" else "本地 cookie 当前不可用",
            }
            self._request("POST", "/api/v1/agent/accounts/sync", payload=payload)
            mirrored += 1

        self._update_state(
            mirroredAccounts=mirrored,
            lastAccountSyncAt=self._now_string(),
        )
        if mirrored:
            agent_logger.debug("omnidrive bridge synced accounts count={} device_code={}", mirrored, self.device_code)
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.accounts_idle:{self.device_code}",
                max(self.account_sync_interval * 2, 120),
                "omnidrive bridge account sync idle device_code={}",
                self.device_code,
            )

    def _sync_materials(self):
        roots = list_material_roots(self.material_roots)
        self._request(
            "POST",
            "/api/v1/agent/materials/roots/sync",
            payload={
                "deviceCode": self.device_code,
                "roots": roots,
            },
        )

        synced_files = 0
        for root_item in roots:
            if synced_files >= self.max_material_files:
                break
            if not root_item.get("exists") or not root_item.get("isDirectory"):
                continue
            synced_files += self._sync_material_directory_tree(root_item["name"], "", synced_files)

        self._update_state(
            syncedRoots=len(roots),
            syncedFiles=synced_files,
            lastMaterialSyncAt=self._now_string(),
        )
        if roots or synced_files:
            agent_logger.debug(
                "omnidrive bridge synced materials roots={} files={} device_code={}",
                len(roots),
                synced_files,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.materials_idle:{self.device_code}",
                max(self.material_sync_interval * 2, 180),
                "omnidrive bridge material sync idle device_code={}",
                self.device_code,
            )

    def _sync_material_directory_tree(self, root_name, relative_path, synced_files):
        if synced_files >= self.max_material_files:
            return 0

        listing = list_material_directory(
            self.material_roots,
            root_name=root_name,
            relative_path=relative_path,
            limit=self.max_material_files,
        )
        self._request(
            "POST",
            "/api/v1/agent/materials/directory/sync",
            payload={
                "deviceCode": self.device_code,
                "root": listing["root"],
                "rootPath": listing["rootPath"],
                "path": listing["path"],
                "absolutePath": listing["absolutePath"],
                "entries": listing["entries"],
            },
        )

        processed_files = 0
        for entry in listing["entries"]:
            if synced_files + processed_files >= self.max_material_files:
                break
            if entry["kind"] == "directory":
                processed_files += self._sync_material_directory_tree(root_name, entry["relativePath"], synced_files + processed_files)
                continue

            file_preview = read_material_file(
                self.material_roots,
                root_name=root_name,
                relative_path=entry["relativePath"],
                max_bytes=self.material_preview_bytes,
            )
            self._request(
                "POST",
                "/api/v1/agent/materials/file/sync",
                payload={
                    "deviceCode": self.device_code,
                    "root": file_preview["root"],
                    "rootPath": file_preview["rootPath"],
                    "path": file_preview["path"],
                    "absolutePath": file_preview["absolutePath"],
                    "name": file_preview["name"],
                    "size": file_preview["size"],
                    "modifiedAt": file_preview["modifiedAt"],
                    "mimeType": file_preview["mimeType"],
                    "isText": file_preview["isText"],
                    "truncated": file_preview["truncated"],
                    "previewText": file_preview["previewText"],
                    "extension": file_preview["extension"],
                },
            )
            processed_files += 1

        return processed_files

    def _sync_skills(self):
        payload = self._request("GET", f"/api/v1/agent/skills/{self.device_code}") or {}
        items = payload.get("items") or []
        retired_items = payload.get("retiredItems") or []
        sync_items = []
        ack_items = []
        synced_count = 0

        for item in items:
            try:
                revision = str(item.get("revision") or "").strip()
                skill = item.get("skill") or {}
                skill_id = str(skill.get("id") or "").strip()
                if not skill_id or not revision:
                    continue

                manifest_dir = self._skill_cache_dir / skill_id
                assets_dir = manifest_dir / "assets"
                assets_dir.mkdir(parents=True, exist_ok=True)

                download_errors = []
                used_file_names = set()
                expected_paths = set()
                enriched_assets = []
                for asset in item.get("assets") or []:
                    asset_payload = dict(asset or {})
                    public_url = self._normalize_cloud_public_url(asset_payload.get("publicUrl"))
                    if public_url:
                        asset_payload["publicUrl"] = public_url
                    file_name = self._allocate_skill_asset_file_name(asset_payload, used_file_names)
                    target_path = assets_dir / file_name
                    metadata_path = assets_dir / f"{file_name}.json"
                    expected_paths.add(target_path.name)
                    expected_paths.add(metadata_path.name)
                    asset_payload["localFileName"] = file_name
                    asset_payload["localPath"] = str(target_path)
                    asset_payload["metadataPath"] = str(metadata_path)
                    asset_payload["downloadStatus"] = "skipped"
                    asset_payload["downloadError"] = None
                    with open(metadata_path, "w", encoding="utf-8") as metadata_file:
                        json.dump(asset_payload, metadata_file, ensure_ascii=False, indent=2)
                    if public_url:
                        try:
                            response = self._session.get(public_url, timeout=self.http_timeout)
                            response.raise_for_status()
                            with open(target_path, "wb") as asset_file:
                                asset_file.write(response.content)
                            asset_payload["downloadStatus"] = "success"
                        except Exception as exc:
                            target_path.unlink(missing_ok=True)
                            error_message = f"{file_name}: {exc}"
                            asset_payload["downloadStatus"] = "failed"
                            asset_payload["downloadError"] = str(exc)
                            download_errors.append(error_message)
                    else:
                        target_path.unlink(missing_ok=True)
                    with open(metadata_path, "w", encoding="utf-8") as metadata_file:
                        json.dump(asset_payload, metadata_file, ensure_ascii=False, indent=2)
                    enriched_assets.append(asset_payload)

                self._cleanup_skill_asset_dir(assets_dir, expected_paths)
                manifest_payload = dict(item)
                manifest_payload["assets"] = enriched_assets
                manifest_payload["cache"] = {
                    "manifestPath": str(manifest_dir / "manifest.json"),
                    "assetsDir": str(assets_dir),
                    "syncedAt": self._iso_now(),
                }
                with open(manifest_dir / "manifest.json", "w", encoding="utf-8") as manifest_file:
                    json.dump(manifest_payload, manifest_file, ensure_ascii=False, indent=2)

                sync_items.append(
                    {
                        "skillId": skill_id,
                        "syncStatus": "success" if not download_errors else "failed",
                        "syncedRevision": revision if not download_errors else None,
                        "assetCount": len(item.get("assets") or []),
                        "message": None if not download_errors else "；".join(download_errors)[:500],
                        "lastSyncedAt": self._iso_now(),
                    }
                )
                if not download_errors:
                    synced_count += 1
            except Exception as exc:
                skill_id = str((item.get("skill") or {}).get("id") or "").strip()
                if skill_id:
                    sync_items.append(
                        {
                            "skillId": skill_id,
                            "syncStatus": "failed",
                            "syncedRevision": None,
                            "assetCount": len(item.get("assets") or []),
                            "message": str(exc)[:500],
                            "lastSyncedAt": self._iso_now(),
                        }
                    )

        if sync_items:
            self._request(
                "POST",
                "/api/v1/agent/skills/sync",
                payload={
                    "deviceCode": self.device_code,
                    "items": sync_items,
                },
            )

        for item in retired_items:
            skill_id = str(item.get("skillId") or "").strip()
            reason = str(item.get("reason") or "").strip()
            if not skill_id or not reason:
                continue
            skill_dir = self._skill_cache_dir / skill_id
            if skill_dir.exists():
                shutil.rmtree(skill_dir, ignore_errors=True)
            ack_items.append(
                {
                    "skillId": skill_id,
                    "reason": reason,
                    "message": "本地技能缓存已清理",
                    "acknowledgedAt": self._iso_now(),
                }
            )

        if ack_items:
            self._request(
                "POST",
                "/api/v1/agent/skills/retired-ack",
                payload={
                    "deviceCode": self.device_code,
                    "items": ack_items,
                },
            )

        self._update_state(
            syncedSkills=synced_count,
            retiredSkillAcks=len(ack_items),
            lastSkillSyncAt=self._now_string(),
        )
        if sync_items or ack_items:
            agent_logger.debug(
                "omnidrive bridge synced skills success_count={} sync_items={} retired_ack_count={} device_code={}",
                synced_count,
                len(sync_items),
                len(ack_items),
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.skills_idle:{self.device_code}",
                max(self.skill_sync_interval * 2, 120),
                "omnidrive bridge skill sync idle device_code={}",
                self.device_code,
            )

    def _poll_remote_login_tasks(self):
        queue_payload = self._request("GET", f"/api/v1/agent/login-tasks/{self.device_code}") or []
        sessions = self._normalize_login_session_queue(queue_payload)
        sessions = self._cleanup_remote_login_sessions(sessions)
        active_remote_sessions = []
        pending_sessions = []
        for session in sessions:
            status = str(session.get("status") or "").strip().lower()
            if status in {"running", "verification_required"}:
                active_remote_sessions.append(session)
            elif status in {"", "pending"}:
                pending_sessions.append(session)

        self._update_state(
            lastLoginPollAt=self._now_string(),
            activeLoginSessionCount=len(sessions),
            activeLoginSessions=[
                self._build_login_session_snapshot(item)
                for item in sessions[:10]
            ],
        )
        if sessions:
            agent_logger.debug(
                "omnidrive bridge polled remote login sessions count={} device_code={}",
                len(sessions),
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.login_idle:{self.device_code}",
                max(self.poll_interval * 20, 60),
                "omnidrive bridge login queue idle device_code={}",
                self.device_code,
            )

        if self._get_active_login_worker():
            return

        if active_remote_sessions:
            first_session = active_remote_sessions[0]
            log_throttled(
                agent_logger,
                "WARNING",
                f"omnidrive_agent.login_resume_blocked:{self.device_code}",
                max(self.poll_interval * 4, 20),
                "omnidrive bridge detected remote login session waiting in cloud and will not auto-restart stale session_id={} status={} device_code={}",
                str(first_session.get("id") or "").strip(),
                str(first_session.get("status") or "").strip(),
                self.device_code,
            )

        seen_targets = set()
        deduplicated_pending_sessions = []
        for session in reversed(pending_sessions):
            target_key = self._build_login_target_key(session)
            if target_key and target_key in seen_targets:
                continue
            if target_key:
                seen_targets.add(target_key)
            deduplicated_pending_sessions.append(session)

        for session in deduplicated_pending_sessions:
            if self._start_login_session(session):
                break

    def _normalize_login_session_queue(self, payload):
        if isinstance(payload, list):
            return payload
        if isinstance(payload, dict):
            items = payload.get("items")
            if isinstance(items, list):
                return items
        return []

    def _build_login_session_snapshot(self, session):
        platform = str(session.get("platform") or "").strip()
        normalized = self._resolve_login_platform(platform)
        return {
            "sessionId": str(session.get("id") or "").strip() or None,
            "platform": platform or None,
            "platformLabel": normalized["label"] if normalized else platform or None,
            "accountName": str(session.get("accountName") or "").strip() or None,
            "status": str(session.get("status") or "").strip() or None,
            "message": self._trim_message(session.get("message")),
            "updatedAt": session.get("updatedAt"),
        }

    @staticmethod
    def _build_login_target_key(session):
        platform = str(session.get("platform") or "").strip().lower()
        account_name = str(session.get("accountName") or "").strip().lower()
        if not platform or not account_name:
            return None
        return f"{platform}:{account_name}"

    @staticmethod
    def _build_login_target_key_from_worker(worker):
        if not isinstance(worker, dict):
            return None
        platform = str(worker.get("platform") or worker.get("platformSlug") or "").strip().lower()
        account_name = str(worker.get("accountName") or "").strip().lower()
        if not platform or not account_name:
            return None
        return f"{platform}:{account_name}"

    def _cleanup_remote_login_sessions(self, sessions):
        if not sessions:
            return []

        active_worker = self._get_active_login_worker()
        active_session_id = ""
        active_target_key = None
        if active_worker:
            active_session_id = str(active_worker.get("sessionId") or "").strip()
            active_target_key = self._build_login_target_key_from_worker(active_worker)

        latest_session_id_by_target = {}
        ordered_sessions = sorted(
            sessions,
            key=self._login_session_sort_key,
            reverse=True,
        )
        for session in ordered_sessions:
            session_id = str(session.get("id") or "").strip()
            if not session_id:
                continue
            target_key = self._build_login_target_key(session)
            if not target_key or target_key in latest_session_id_by_target:
                continue
            latest_session_id_by_target[target_key] = session_id

        if active_target_key and active_session_id:
            latest_session_id_by_target[active_target_key] = active_session_id

        cancelled_session_ids = set()
        kept_sessions = []
        for session in sessions:
            session_id = str(session.get("id") or "").strip()
            if not session_id:
                continue
            if session_id == active_session_id:
                kept_sessions.append(session)
                continue

            target_key = self._build_login_target_key(session)
            status = str(session.get("status") or "").strip().lower()

            startup_cleanup_message = self._get_startup_orphan_login_session_message(session)
            if startup_cleanup_message:
                if self._cancel_remote_login_session(
                    session,
                    status="cancelled",
                    message=startup_cleanup_message,
                ):
                    cancelled_session_ids.add(session_id)
                continue

            if active_target_key and target_key and target_key == active_target_key:
                if self._cancel_remote_login_session(
                    session,
                    status="cancelled",
                    message="本地 SAU 已清理同账号的重复登录会话，请继续当前登录窗口或重新发起登录。",
                ):
                    cancelled_session_ids.add(session_id)
                continue

            latest_session_id = latest_session_id_by_target.get(target_key) if target_key else None
            if target_key and latest_session_id and latest_session_id != session_id and status in {"pending", "running", "verification_required"}:
                if self._cancel_remote_login_session(
                    session,
                    status="cancelled",
                    message="本地 SAU 已清理被更新登录请求覆盖的旧会话，请使用最新登录窗口继续操作。",
                ):
                    cancelled_session_ids.add(session_id)
                continue

            stale_message = self._get_stale_login_session_message(session)
            if stale_message:
                if self._cancel_remote_login_session(
                    session,
                    status="cancelled",
                    message=stale_message,
                ):
                    cancelled_session_ids.add(session_id)
                continue

            kept_sessions.append(session)

        if cancelled_session_ids:
            agent_logger.info(
                "omnidrive bridge cleaned stale login sessions count={} device_code={}",
                len(cancelled_session_ids),
                self.device_code,
            )
        self._login_startup_cleanup_done = True
        return kept_sessions

    def _cancel_remote_login_session(self, session, *, status, message):
        session_id = str(session.get("id") or "").strip()
        if not session_id:
            return False
        try:
            self._post_login_event(
                session_id,
                status=status,
                message=message,
            )
            agent_logger.warning(
                "omnidrive bridge cancelled stale login session session_id={} login_status={} platform={} account_name={} device_code={}",
                session_id,
                str(session.get("status") or "").strip(),
                str(session.get("platform") or "").strip(),
                str(session.get("accountName") or "").strip(),
                self.device_code,
            )
            return True
        except Exception as exc:
            agent_logger.warning(
                "omnidrive bridge failed to cancel stale login session session_id={} error={}",
                session_id,
                exc,
            )
            return False

    def _get_stale_login_session_message(self, session):
        status = str(session.get("status") or "").strip().lower()
        age_seconds = self._get_login_session_age_seconds(session)
        if age_seconds is None:
            return None
        if status in {"", "pending"} and age_seconds >= LOGIN_PENDING_STALE_SECONDS:
            return "本地 SAU 已清理陈旧的待登录会话，请在 OmniDrive 重新发起账号登录。"
        if status == "running" and age_seconds >= LOGIN_RUNNING_STALE_SECONDS:
            return "本地 SAU 已清理失去本地执行窗口的陈旧登录会话，请重新发起账号登录。"
        if status == "verification_required" and age_seconds >= LOGIN_VERIFICATION_STALE_SECONDS:
            return "本地 SAU 已清理失去本地验证窗口的陈旧二次认证会话，请重新发起账号登录。"
        return None

    def _get_startup_orphan_login_session_message(self, session):
        if self._login_startup_cleanup_done:
            return None
        status = str(session.get("status") or "").strip().lower()
        if status not in {"", "pending", "running", "verification_required"}:
            return None
        parsed = self._parse_remote_datetime(session.get("updatedAt") or session.get("createdAt"))
        if parsed is None:
            return None
        startup_cutoff = self._agent_started_at_epoch - max(float(self.poll_interval), 5.0)
        if parsed.timestamp() >= startup_cutoff:
            return None
        if status == "verification_required":
            return "本地 SAU 重启后已清理上次遗留的二次认证会话，请在 OmniDrive 重新发起账号登录。"
        if status == "running":
            return "本地 SAU 重启后已清理上次遗留的登录会话，请在 OmniDrive 重新发起账号登录。"
        return "本地 SAU 重启后已清理上次遗留的待登录会话，请在 OmniDrive 重新发起账号登录。"

    def _get_login_session_age_seconds(self, session):
        reference = session.get("updatedAt") or session.get("createdAt")
        parsed = self._parse_remote_datetime(reference)
        if parsed is None:
            return None
        return max(0.0, time.time() - parsed.timestamp())

    def _login_session_sort_key(self, session):
        parsed = self._parse_remote_datetime(session.get("updatedAt") or session.get("createdAt"))
        return parsed.timestamp() if parsed is not None else 0.0

    @staticmethod
    def _parse_remote_datetime(value):
        if value in (None, "", 0, "0"):
            return None
        if isinstance(value, datetime):
            parsed = value
        else:
            try:
                parsed = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
            except ValueError:
                return None
        try:
            return parsed.astimezone()
        except ValueError:
            return parsed

    def _resolve_login_platform(self, platform):
        alias = str(platform or "").strip().lower()
        if not alias:
            return None
        return LOGIN_PLATFORM_ALIAS_MAP.get(alias)

    def _get_active_login_worker(self):
        with self._login_worker_lock:
            return self._active_login_worker

    def _set_active_login_worker(self, worker):
        with self._login_worker_lock:
            self._active_login_worker = worker

    def _clear_active_login_worker(self, session_id=None):
        with self._login_worker_lock:
            if session_id and self._active_login_worker:
                active_id = str(self._active_login_worker.get("sessionId") or "").strip()
                if active_id and active_id != session_id:
                    return
            self._active_login_worker = None

    def _start_login_session(self, session):
        session_id = str(session.get("id") or "").strip()
        platform = str(session.get("platform") or "").strip()
        account_name = str(session.get("accountName") or "").strip()
        if not session_id or not platform or not account_name:
            return False

        platform_config = self._resolve_login_platform(platform)
        if not platform_config:
            self._post_login_event(
                session_id,
                status="failed",
                message=f"本地 SAU 暂不支持 {platform} 登录",
            )
            return False
        if not callable(self.run_login_fn):
            self._post_login_event(
                session_id,
                status="failed",
                message="本地 SAU 未配置登录执行器，无法拉起账号登录",
            )
            return False

        status_queue = Queue()
        command_queue = Queue()
        worker = {
            "sessionId": session_id,
            "platform": platform,
            "platformSlug": platform_config["slug"],
            "platformLabel": platform_config["label"],
            "accountName": account_name,
            "statusQueue": status_queue,
            "commandQueue": command_queue,
            "lastStatus": str(session.get("status") or "pending").strip() or "pending",
            "lastMessage": self._trim_message(session.get("message")),
            "lastQRData": None,
            "lastVerificationSignature": None,
            "lastVerificationPayload": None,
            "pendingStatusEvents": [],
            "startedAt": self._iso_now(),
        }

        thread = threading.Thread(
            target=self.run_login_fn,
            args=(str(platform_config["type"]), account_name, status_queue, command_queue),
            daemon=True,
        )
        worker["thread"] = thread
        self._set_active_login_worker(worker)

        try:
            thread.start()
        except Exception as exc:
            self._clear_active_login_worker(session_id)
            self._post_login_event(
                session_id,
                status="failed",
                message=f"本地 SAU 拉起登录窗口失败: {exc}",
            )
            return False

        agent_logger.info(
            "omnidrive bridge login session started session_id={} platform={} account_name={}",
            session_id,
            platform_config["label"],
            account_name,
        )
        self._post_login_event(
            session_id,
            status="running",
            message=f"本地 SAU 已拉起 {platform_config['label']} 登录窗口",
        )
        return True

    def _sync_active_login_session(self):
        worker = self._get_active_login_worker()
        if not worker:
            return

        session_id = str(worker.get("sessionId") or "").strip()
        if not session_id:
            self._clear_active_login_worker()
            return

        self._consume_login_actions(worker)
        self._drain_login_status_queue(worker)

        thread = worker.get("thread")
        is_alive = bool(thread and thread.is_alive())
        try:
            queue_empty = worker["statusQueue"].empty()
        except Exception:
            queue_empty = True

        if not is_alive and queue_empty and worker.get("lastStatus") not in FINAL_LOGIN_STATUSES:
            self._post_login_event(
                session_id,
                status="failed",
                message="本地登录线程已结束，但没有返回成功状态",
            )

        if not is_alive and queue_empty and worker.get("lastStatus") in FINAL_LOGIN_STATUSES:
            self._clear_active_login_worker(session_id)

    def _consume_login_actions(self, worker):
        session_id = str(worker.get("sessionId") or "").strip()
        if not session_id:
            return

        actions = self._request("GET", f"/api/v1/agent/login-sessions/{session_id}/actions") or []
        if not isinstance(actions, list):
            return

        command_queue = worker.get("commandQueue")
        if command_queue is None:
            return

        for action in actions:
            action_type = str(action.get("actionType") or "").strip()
            if not action_type:
                continue
            command_queue.put(
                {
                    "actionType": action_type,
                    "payload": action.get("payload") or {},
                }
            )

    def _drain_login_status_queue(self, worker):
        status_queue = worker.get("statusQueue")
        if status_queue is None:
            return

        pending_events = list(worker.get("pendingStatusEvents") or [])
        if pending_events:
            worker["pendingStatusEvents"] = []
            for event in pending_events:
                try:
                    self._handle_login_status_event(worker, event)
                except Exception as exc:
                    worker.setdefault("pendingStatusEvents", []).insert(0, event)
                    raise RuntimeError(f"retry login status event failed: {exc}") from exc

        while True:
            try:
                event = status_queue.get_nowait()
            except Empty:
                break

            try:
                self._handle_login_status_event(worker, event)
            except Exception as exc:
                worker.setdefault("pendingStatusEvents", []).append(event)
                raise RuntimeError(f"process login status event failed: {exc}") from exc

    def _handle_login_status_event(self, worker, event):
        session_id = str(worker.get("sessionId") or "").strip()
        platform_label = str(worker.get("platformLabel") or worker.get("platform") or "账号").strip()
        if not session_id:
            return

        if isinstance(event, dict):
            event_type = str(event.get("type") or "").strip()
            payload = event.get("payload") or {}
            if event_type == "qr_updated":
                qr_data = payload.get("qrData")
                message = self._trim_message(payload.get("message")) or f"{platform_label} 登录二维码已更新"
                if qr_data == worker.get("lastQRData") and message == worker.get("lastMessage"):
                    return
                self._post_login_event(
                    session_id,
                    status="running",
                    message=message,
                    qr_data=qr_data or worker.get("lastQRData"),
                )
                return
            if event_type == "qr_expired":
                qr_data = payload.get("qrData") or worker.get("lastQRData")
                message = self._trim_message(payload.get("message")) or f"{platform_label} 登录二维码已过期，请刷新二维码"
                if message == worker.get("lastMessage") and qr_data == worker.get("lastQRData"):
                    return
                self._post_login_event(
                    session_id,
                    status="running",
                    message=message,
                    qr_data=qr_data,
                )
                return
            if event_type == "verification_required":
                signature = self._derive_login_verification_signature(payload)
                if signature and signature == worker.get("lastVerificationSignature"):
                    return
                worker["lastVerificationSignature"] = signature
                worker["lastVerificationPayload"] = payload
                self._post_login_event(
                    session_id,
                    status="verification_required",
                    message=self._trim_message(payload.get("message")) or f"{platform_label} 登录需要额外验证",
                    verification_payload=payload,
                )
                return
            if event_type == "log":
                message = self._trim_message(payload.get("message"))
                if not message or message == worker.get("lastMessage"):
                    return
                self._post_login_event(
                    session_id,
                    status=worker.get("lastStatus") if worker.get("lastStatus") in {"running", "verification_required"} else "running",
                    message=message,
                )
                return
            return

        message = str(event or "").strip()
        if not message:
            return

        if message == "200":
            self._post_login_event(
                session_id,
                status="success",
                message=f"{platform_label} 账号登录成功，本地 SAU 已保存最新 Cookie",
            )
            try:
                self._sync_accounts()
            except Exception as exc:
                agent_logger.warning(
                    "omnidrive bridge immediate account sync failed session_id={} account_name={} error={}",
                    session_id,
                    worker.get("accountName"),
                    exc,
                )
            return

        if message == "500":
            self._post_login_event(
                session_id,
                status="failed",
                message=f"{platform_label} 账号登录失败，请检查本地 SAU 日志",
            )
            return

        if message == "CANCELLED":
            self._post_login_event(
                session_id,
                status="cancelled",
                message=f"{platform_label} 登录会话已取消，本地 SAU 已停止当前登录流程",
            )
            return

        if message.startswith("data:image"):
            if message == worker.get("lastQRData"):
                return
            worker["lastQRData"] = message
            self._post_login_event(
                session_id,
                status="running",
                message=f"{platform_label} 登录二维码已生成，请在本地浏览器完成扫码",
                qr_data=message,
            )
            return

        if message != worker.get("lastMessage"):
            self._post_login_event(
                session_id,
                status=worker.get("lastStatus") if worker.get("lastStatus") in {"running", "verification_required"} else "running",
                message=message,
            )

    def _derive_login_verification_signature(self, payload):
        if not isinstance(payload, dict):
            return None
        signature = str(payload.get("signature") or "").strip()
        if signature:
            return signature
        title = str(payload.get("title") or "").strip()
        message = str(payload.get("message") or "").strip()
        options = payload.get("options") if isinstance(payload.get("options"), list) else []
        hints = payload.get("inputHints") if isinstance(payload.get("inputHints"), list) else []
        return "|".join(
            [
                title,
                message,
                "/".join(str(item).strip() for item in options if str(item).strip()),
                "/".join(str(item).strip() for item in hints if str(item).strip()),
            ]
        )

    def _post_login_event(self, session_id, *, status, message=None, qr_data=None, verification_payload=None):
        worker = self._get_active_login_worker()
        if worker and str(worker.get("sessionId") or "").strip() == session_id:
            if qr_data is None and str(status or "").strip() in {"running", "verification_required"}:
                qr_data = worker.get("lastQRData")
            if verification_payload is None and str(status or "").strip() == "verification_required":
                verification_payload = worker.get("lastVerificationPayload")
        payload = {
            "status": str(status or "").strip(),
            "message": self._trim_message(message),
            "qrData": qr_data,
            "verificationPayload": verification_payload,
        }
        updated = self._request(
            "POST",
            f"/api/v1/agent/login-sessions/{session_id}/event",
            payload=payload,
        ) or {}
        agent_logger.debug(
            "omnidrive bridge login event pushed session_id={} status={} has_qr={} has_verification={}",
            session_id,
            payload["status"],
            bool(qr_data),
            verification_payload is not None,
        )
        self._update_state(
            lastLoginEventAt=self._now_string(),
            lastError=None,
        )

        if worker and str(worker.get("sessionId") or "").strip() == session_id:
            worker["lastStatus"] = payload["status"]
            worker["lastMessage"] = payload["message"]
            if qr_data:
                worker["lastQRData"] = qr_data
            if verification_payload is not None:
                worker["lastVerificationPayload"] = verification_payload

        return updated

    @staticmethod
    def _trim_message(value):
        text = str(value or "").strip()
        return text or None

    def _import_remote_publish_tasks(self):
        queue_payload = self._request("GET", f"/api/v1/agent/publish-tasks/{self.device_code}") or []
        queue = self._normalize_publish_task_queue(queue_payload)
        imported = 0
        last_error = None

        inflight = self._count_inflight_omnidrive_tasks()
        available_slots = max(0, self.publish_task_manager.worker_count - inflight)
        if available_slots <= 0:
            self._update_state(lastPublishPollAt=self._now_string())
            return

        for task in queue:
            if imported >= available_slots:
                break

            task_id = str(task.get("id") or "").strip()
            if not task_id:
                continue
            if self.publish_task_manager.get_task(task_id):
                continue

            try:
                package = self._request(
                    "GET",
                    f"/api/v1/agent/publish-tasks/{task_id}/package",
                    params={"deviceCode": self.device_code},
                )
            except Exception as exc:
                last_error = f"OmniDrive 任务包获取失败 {task_id}: {self._format_remote_error(exc)}"
                self._update_state(lastError=last_error)
                continue
            if not package:
                continue

            try:
                claim_data = self._request(
                    "POST",
                    f"/api/v1/agent/publish-tasks/{task_id}/claim",
                    payload={"deviceCode": self.device_code},
                )
            except Exception as exc:
                last_error = f"OmniDrive 任务认领失败 {task_id}: {self._format_remote_error(exc)}"
                self._update_state(lastError=last_error)
                continue
            lease_token = str((claim_data or {}).get("leaseToken") or "").strip()
            lease_expires_at = (claim_data or {}).get("leaseExpiresAt")
            if not lease_token:
                last_error = f"OmniDrive 任务认领未返回租约 {task_id}"
                self._update_state(lastError=last_error)
                continue

            try:
                local_spec = self._build_local_task_spec(package)
                self.publish_task_manager.enqueue_specs([local_spec])
                self._upsert_lease(task_id, lease_token, lease_expires_at)
                self._sync_imported_task_runtime(local_spec)
                imported += 1
            except Exception as exc:
                last_error = f"OmniDrive 任务导入失败 {task_id}: {exc}"
                self._update_state(lastError=last_error)
                self._sync_remote_import_failure(package, lease_token, str(exc))
                self._delete_lease(task_id)

        self._update_state(
            importedCloudTasks=imported,
            lastPublishPollAt=self._now_string(),
        )
        if imported:
            agent_logger.info(
                "omnidrive bridge imported remote publish tasks count={} device_code={}",
                imported,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.publish_poll_idle:{self.device_code}",
                max(self.poll_interval * 20, 60),
                "omnidrive bridge publish queue idle device_code={}",
                self.device_code,
            )
        if last_error:
            self._update_state(lastError=last_error)
            log_throttled(
                agent_logger,
                "WARNING",
                f"omnidrive_agent.publish_import_error:{self.device_code}",
                30,
                "omnidrive bridge remote publish import error device_code={} error={}",
                self.device_code,
                last_error,
            )

    def _sync_imported_task_runtime(self, local_spec):
        task_uuid = local_spec["taskUuid"]
        binding = self._get_lease(task_uuid)
        if not binding:
            return
        payload = {
            "id": task_uuid,
            "deviceCode": self.device_code,
            "accountId": local_spec["payload"].get("omnidriveAccountId"),
            "skillId": local_spec["payload"].get("omnidriveSkillId"),
            "skillRevision": local_spec["payload"].get("omnidriveSkillRevision"),
            "platform": local_spec["platformName"],
            "accountName": local_spec["accountName"],
            "title": local_spec["title"],
            "contentText": local_spec["payload"].get("contentText"),
            "mediaPayload": local_spec["payload"].get("omnidriveMediaPayload"),
            "status": "running",
            "message": "任务已进入本地执行队列",
            "executionPayload": {
                "stage": "queued_local",
                "source": local_spec["source"],
                "queuedAt": self._iso_now(),
            },
            "materialRefs": local_spec["payload"].get("omnidriveMaterialRefs") or [],
            "leaseToken": binding["lease_token"],
        }
        self._request("POST", "/api/v1/agent/publish-tasks/sync", payload=payload)

    def _sync_local_publish_tasks(self):
        tasks = self.publish_task_manager.list_tasks(limit=500, sources=SYNCABLE_LOCAL_SOURCES)
        mirrored = 0
        last_error = None
        for task in tasks:
            if not self._should_sync_local_task(task):
                continue
            payload = self._build_sync_task_payload(task)
            if not payload:
                continue
            try:
                self._request("POST", "/api/v1/agent/publish-tasks/sync", payload=payload)
            except Exception as exc:
                if self._handle_sync_local_publish_task_error(task, exc):
                    continue
                last_error = f"OmniDrive 本地任务同步失败 {task.get('taskUuid')}: {self._format_remote_error(exc)}"
                continue
            if task["status"] in FINAL_LOCAL_STATUSES:
                self._finalize_lease_record(task["taskUuid"], task.get("status"), task.get("updatedAt"))
            else:
                self._mark_task_synced(task)
            if task["status"] in FINAL_LOCAL_STATUSES:
                self._clear_lease_token(task["taskUuid"])
            mirrored += 1

        self._update_state(
            mirroredTasks=mirrored,
            lastPublishSyncAt=self._now_string(),
        )
        if mirrored:
            agent_logger.debug(
                "omnidrive bridge synced local publish tasks count={} device_code={}",
                mirrored,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.publish_sync_idle:{self.device_code}",
                max(self.publish_sync_interval * 20, 60),
                "omnidrive bridge local publish sync idle device_code={}",
                self.device_code,
            )
        if last_error:
            self._update_state(lastError=last_error)
            log_throttled(
                agent_logger,
                "WARNING",
                f"omnidrive_agent.publish_sync_error:{self.device_code}",
                30,
                "omnidrive bridge local publish sync error device_code={} error={}",
                self.device_code,
                last_error,
            )

    def _handle_sync_local_publish_task_error(self, task, exc):
        response = getattr(exc, "response", None)
        status_code = getattr(response, "status_code", None)
        if status_code != 409:
            return False

        if str(task.get("status") or "").strip() not in FINAL_LOCAL_STATUSES:
            return False

        message = self._extract_remote_error_message(response)
        if message != "Publish task belongs to a different device":
            return False

        self._finalize_lease_record(task["taskUuid"], task.get("status"), task.get("updatedAt"))
        self._clear_lease_token(task["taskUuid"])
        return True

    def _sync_local_ai_tasks(self):
        if not self.ai_task_manager:
            return 0
        tasks = self.ai_task_manager.list_tasks_for_cloud_sync(limit=200)
        mirrored = 0
        for task in tasks:
            source = str(task.get("source") or "").strip() or "local_ui"
            if source not in SYNCABLE_LOCAL_AI_SOURCES:
                continue
            payload = self._build_sync_ai_task_payload(task)
            if not payload:
                continue
            data = self._request("POST", "/api/v1/agent/ai-jobs/sync", payload=payload) or {}
            job = data.get("job") or {}
            cloud_job_id = str(job.get("id") or "").strip()
            cloud_status = str(job.get("status") or "queued").strip() or "queued"
            message = job.get("message") or "AI 任务已同步到 OmniDrive 云端"
            if cloud_job_id:
                self.ai_task_manager.update_cloud_binding(task["taskUuid"], cloud_job_id, cloud_status, message)
                mirrored += 1
        self._update_state(
            mirroredAITasks=mirrored,
            lastAISyncAt=self._now_string(),
        )
        if mirrored:
            agent_logger.debug(
                "omnidrive bridge synced local ai tasks count={} device_code={}",
                mirrored,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.ai_sync_idle:{self.device_code}",
                max(self.publish_sync_interval * 20, 60),
                "omnidrive bridge local ai sync idle device_code={}",
                self.device_code,
            )
        return mirrored

    def _import_missing_remote_ai_task(self, job):
        if not self.ai_task_manager:
            return None

        local_task_id = str(job.get("localTaskId") or "").strip()
        cloud_job_id = str(job.get("id") or "").strip()
        if not local_task_id or not cloud_job_id:
            return None

        payload = job.get("inputPayload") or {}
        if not isinstance(payload, dict):
            payload = {}

        imported = self.ai_task_manager.import_remote_task(
            {
                "taskUuid": local_task_id,
                "source": str(job.get("source") or "omnidrive_cloud").strip() or "omnidrive_cloud",
                "jobType": str(job.get("jobType") or "").strip(),
                "modelName": str(job.get("modelName") or "").strip(),
                "skillId": str(job.get("skillId") or "").strip() or None,
                "prompt": str(job.get("prompt") or "").strip(),
                "status": self._local_ai_status_from_remote_job(job),
                "cloudStatus": str(job.get("status") or "").strip() or "queued",
                "message": job.get("deliveryMessage") or job.get("message"),
                "payload": payload,
                "cloudJobId": cloud_job_id,
                "linkedPublishTaskUuid": str(job.get("localPublishTaskId") or "").strip() or None,
                "artifactRefs": [],
            }
        )
        agent_logger.info(
            "omnidrive bridge imported remote ai task record task_uuid={} cloud_job_id={} status={}",
            local_task_id,
            cloud_job_id,
            imported.get("status") if imported else None,
        )
        return imported

    def _import_remote_ai_jobs(self):
        if not self.ai_task_manager:
            return 0
        items = self._request(
            "GET",
            f"/api/v1/agent/ai-jobs/{self.device_code}",
            params={"limit": 200},
        ) or []
        imported = 0
        for item in items:
            job = item.get("job") or {}
            artifacts = item.get("artifacts") or []
            cloud_job_id = str(job.get("id") or "").strip()
            local_task_id = str(job.get("localTaskId") or "").strip()
            cloud_status = str(job.get("status") or "").strip()
            if not cloud_job_id or not local_task_id:
                continue

            local_task = self.ai_task_manager.get_task(local_task_id)
            if not local_task:
                local_task = self._import_missing_remote_ai_task(job)
                if not local_task:
                    continue

            self.ai_task_manager.update_cloud_binding(
                local_task_id,
                cloud_job_id,
                cloud_status or local_task.get("cloudStatus") or "queued",
                job.get("message"),
            )

            if cloud_status in {"queued", "running"}:
                continue

            if cloud_status in {"failed", "cancelled"}:
                self.ai_task_manager.mark_cloud_state(local_task_id, cloud_status, job.get("message"))
                continue

            if cloud_status not in {"success", "completed"}:
                continue

            if local_task.get("linkedPublishTaskUuid"):
                continue

            artifact_refs = self._download_ai_artifacts(local_task, artifacts)
            if not artifact_refs:
                self.ai_task_manager.mark_cloud_state(local_task_id, "failed", "云端 AI 任务没有可导入的有效产物")
                self._request(
                    "POST",
                    f"/api/v1/agent/ai-jobs/{cloud_job_id}/delivery",
                    payload={
                        "deviceCode": self.device_code,
                        "status": "failed",
                        "message": "云端 AI 任务没有可导入的有效产物",
                        "deliveredAt": self._iso_now(),
                    },
                )
                continue

            publish_task_uuid = self._enqueue_publish_from_ai_task(local_task, artifact_refs)
            if publish_task_uuid:
                self.ai_task_manager.mark_result_imported(
                    local_task_id,
                    artifact_refs,
                    linked_publish_task_uuid=publish_task_uuid,
                    message="AI 产物已回流 OmniBull，并进入 SAU 发布队列",
                )
                self._request(
                    "POST",
                    f"/api/v1/agent/ai-jobs/{cloud_job_id}/delivery",
                    payload={
                        "deviceCode": self.device_code,
                        "status": "publish_queued",
                        "message": "AI 产物已回流 OmniBull，并进入 SAU 发布队列",
                        "localPublishTaskId": publish_task_uuid,
                        "deliveredAt": self._iso_now(),
                    },
                )
            else:
                self.ai_task_manager.mark_result_imported(
                    local_task_id,
                    artifact_refs,
                    message="AI 产物已回流 OmniBull，本地尚未生成发布任务",
                )
                self._request(
                    "POST",
                    f"/api/v1/agent/ai-jobs/{cloud_job_id}/delivery",
                    payload={
                        "deviceCode": self.device_code,
                        "status": "imported",
                        "message": "AI 产物已回流 OmniBull，本地尚未生成发布任务",
                        "deliveredAt": self._iso_now(),
                    },
                )
            imported += 1

        self._update_state(
            importedAIResults=imported,
            lastAIPollAt=self._now_string(),
        )
        if imported:
            agent_logger.info(
                "omnidrive bridge imported remote ai results count={} device_code={}",
                imported,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.ai_poll_idle:{self.device_code}",
                max(self.poll_interval * 20, 60),
                "omnidrive bridge remote ai queue idle device_code={}",
                self.device_code,
            )
        return imported

    def _sync_local_ai_publish_state(self):
        if not self.ai_task_manager:
            return 0
        tasks = self.ai_task_manager.list_tasks(limit=500)
        synced = 0
        for task in tasks:
            linked_publish_task_uuid = str(task.get("linkedPublishTaskUuid") or "").strip()
            cloud_job_id = str(task.get("cloudJobId") or "").strip()
            if not linked_publish_task_uuid or not cloud_job_id:
                continue
            publish_task = self.publish_task_manager.get_task(linked_publish_task_uuid)
            if not publish_task:
                continue
            publish_status = str(publish_task.get("status") or "").strip()
            current_status = str(task.get("status") or "").strip()
            desired_status = self._map_publish_status_to_local_ai_status(publish_status)
            if desired_status and desired_status != current_status:
                message = publish_task.get("message") or "本地发布任务状态已更新"
                self.ai_task_manager.sync_linked_publish_status(task["taskUuid"], publish_status, message)
                self._request(
                    "POST",
                    f"/api/v1/agent/ai-jobs/{cloud_job_id}/delivery",
                    payload={
                        "deviceCode": self.device_code,
                        "status": self._map_publish_status_to_delivery_status(publish_status),
                        "message": message,
                        "localPublishTaskId": linked_publish_task_uuid,
                        "deliveredAt": self._iso_now(),
                    },
                )
                synced += 1
        return synced

    def _renew_active_leases(self):
        renewed = 0
        for binding in self._list_leases():
            if not binding.get("lease_token"):
                continue
            task = self.publish_task_manager.get_task(binding["task_uuid"])
            if not task:
                self._delete_lease(binding["task_uuid"])
                continue
            if task["status"] in FINAL_LOCAL_STATUSES:
                continue
            if not self._lease_needs_renew(binding["lease_expires_at"]):
                continue
            data = self._request(
                "POST",
                f"/api/v1/agent/publish-tasks/{binding['task_uuid']}/renew",
                payload={
                    "deviceCode": self.device_code,
                    "leaseToken": binding["lease_token"],
                },
            )
            self._upsert_lease(
                binding["task_uuid"],
                binding["lease_token"],
                (data or {}).get("leaseExpiresAt"),
                last_synced_status=binding.get("last_synced_status"),
                last_synced_updated_at=binding.get("last_synced_updated_at"),
            )
            remote_task = (data or {}).get("task") or {}
            if remote_task.get("status") == "cancel_requested" and task.get("status") in {"pending", "scheduled"}:
                cancel_message = "OmniDrive 请求取消，本地任务尚未开始，已终止排队"
                if self.publish_task_manager.cancel_task_if_queued(binding["task_uuid"], cancel_message):
                    self._request(
                        "POST",
                        f"/api/v1/agent/publish-tasks/{binding['task_uuid']}/release",
                        payload={
                            "deviceCode": self.device_code,
                            "leaseToken": binding["lease_token"],
                            "message": cancel_message,
                        },
                    )
                    cancelled_task = self.publish_task_manager.get_task(binding["task_uuid"])
                    if cancelled_task:
                        self._finalize_lease_record(
                            binding["task_uuid"],
                            cancelled_task.get("status"),
                            cancelled_task.get("updatedAt"),
                        )
                        self._clear_lease_token(binding["task_uuid"])
            renewed += 1

        self._update_state(
            lastLeaseRenewAt=self._now_string(),
            lastError=None,
        )
        if renewed:
            agent_logger.debug(
                "omnidrive bridge renewed active leases count={} device_code={}",
                renewed,
                self.device_code,
            )
        else:
            log_throttled(
                agent_logger,
                "DEBUG",
                f"omnidrive_agent.lease_idle:{self.device_code}",
                max(self.publish_sync_interval * 20, 60),
                "omnidrive bridge lease renew idle device_code={}",
                self.device_code,
            )
        return renewed

    def _build_local_task_spec(self, package):
        task = package.get("task") or {}
        task_id = str(task.get("id") or "").strip()
        platform_name = str(task.get("platform") or "").strip()
        platform_type = PLATFORM_TYPE_BY_NAME.get(platform_name)
        if not task_id or not platform_type:
            raise ValueError("云端任务缺少有效的平台信息")

        account_file_path = self._resolve_local_account_file_path(platform_name, str(task.get("accountName") or "").strip())
        media_payload = package.get("task", {}).get("mediaPayload") or {}
        material_refs = self._extract_material_refs_from_package(package)
        primary_file = self._choose_primary_file(material_refs)
        if not primary_file:
            raise ValueError("云端任务没有可执行的视频素材")

        publish_date = self._extract_publish_date(task, media_payload)
        thumbnail_ref = self._choose_thumbnail(material_refs, media_payload)
        tags = self._normalize_tags(media_payload.get("tags"))
        title = str(task.get("title") or "").strip()
        if not title:
            raise ValueError("云端任务标题不能为空")

        payload = {
            "platformType": platform_type,
            "platformName": platform_name,
            "title": title,
            "tags": tags,
            "filePath": primary_file["displayPath"],
            "accountFilePath": account_file_path,
            "accountName": str(task.get("accountName") or "").strip(),
            "publishDate": publish_date or 0,
            "category": media_payload.get("category"),
            "fileSourceMode": "material",
            "materialRoot": primary_file["root"],
            "materialPath": primary_file["path"],
            "sourceAbsolutePath": primary_file["absolutePath"],
            "productLink": media_payload.get("productLink") or "",
            "productTitle": media_payload.get("productTitle") or "",
            "isDraft": bool(media_payload.get("isDraft", False)),
            "source": "omnidrive_agent",
            "contentText": task.get("contentText"),
            "omnidriveAccountId": task.get("accountId"),
            "omnidriveSkillId": task.get("skillId"),
            "omnidriveSkillRevision": task.get("skillRevision"),
            "omnidriveMediaPayload": media_payload,
            "omnidriveMaterialRefs": material_refs,
        }
        if thumbnail_ref:
            payload.update(
                {
                    "thumbnailSourceMode": "material",
                    "thumbnailRoot": thumbnail_ref["root"],
                    "thumbnailPath": thumbnail_ref["path"],
                    "thumbnailAbsolutePath": thumbnail_ref["absolutePath"],
                }
            )

        run_at = self._normalize_datetime(task.get("runAt"))
        status = "scheduled" if self._is_future_datetime(run_at) else "pending"
        message = "等待定时执行" if status == "scheduled" else "等待 OmniBull worker 执行"
        return {
            "taskUuid": task_id,
            "source": "omnidrive_agent",
            "platformType": platform_type,
            "platformName": platform_name,
            "accountName": str(task.get("accountName") or "").strip(),
            "accountFilePath": account_file_path,
            "fileName": primary_file["name"],
            "filePath": primary_file["displayPath"],
            "title": title,
            "runAt": run_at,
            "platformPublishAt": publish_date,
            "status": status,
            "message": message,
            "payload": payload,
        }

    def _sync_remote_import_failure(self, package, lease_token, error_message):
        task = package.get("task") or {}
        payload = {
            "id": str(task.get("id") or "").strip(),
            "deviceCode": self.device_code,
            "accountId": task.get("accountId"),
            "skillId": task.get("skillId"),
            "skillRevision": task.get("skillRevision"),
            "platform": str(task.get("platform") or "").strip(),
            "accountName": str(task.get("accountName") or "").strip(),
            "title": str(task.get("title") or "").strip(),
            "contentText": task.get("contentText"),
            "mediaPayload": task.get("mediaPayload") or {},
            "status": "failed",
            "message": f"OmniBull 本地预处理失败: {error_message}",
            "executionPayload": {
                "stage": "preflight_failed",
                "deviceCode": self.device_code,
                "failedAt": self._iso_now(),
            },
            "materialRefs": self._extract_material_refs_from_package(package),
            "leaseToken": lease_token,
        }
        self._request("POST", "/api/v1/agent/publish-tasks/sync", payload=payload)

    def _build_sync_task_payload(self, task):
        payload = task.get("payload") or {}
        source = str(task.get("source") or "").strip()
        status = str(task.get("status") or "").strip()
        task_uuid = str(task.get("taskUuid") or "").strip()
        if not task_uuid or not status:
            return None

        binding = self._get_lease(task_uuid)
        if source == "omnidrive_agent" and status in {"pending", "scheduled"} and binding:
            return None

        remote_status = self._map_local_status(status)
        if not remote_status:
            return None

        verification_payload = dict(task.get("verificationData") or {})
        if verification_payload and not verification_payload.get("screenshotData") and task.get("artifactPath"):
            screenshot_data = self._artifact_file_to_data_url(task.get("artifactPath"))
            if screenshot_data:
                verification_payload["screenshotData"] = screenshot_data

        media_payload = self._build_remote_media_payload(task)
        artifacts = self._build_remote_artifacts(task, verification_payload)
        sync_payload = {
            "id": task_uuid,
            "deviceCode": self.device_code,
            "accountId": payload.get("omnidriveAccountId"),
            "skillId": payload.get("omnidriveSkillId"),
            "skillRevision": payload.get("omnidriveSkillRevision"),
            "platform": task.get("platformName"),
            "accountName": task.get("accountName"),
            "title": task.get("title"),
            "contentText": payload.get("contentText"),
            "mediaPayload": media_payload,
            "status": remote_status,
            "message": task.get("message"),
            "executionPayload": {
                "stage": self._runtime_stage_for_status(status),
                "workerName": task.get("workerName"),
                "localStatus": status,
                "source": source,
                "startedAt": task.get("startedAt"),
                "finishedAt": task.get("finishedAt"),
                "updatedAt": task.get("updatedAt"),
            },
            "verificationPayload": verification_payload or None,
            "artifacts": artifacts,
            "runAt": self._to_rfc3339(task.get("runAt")),
            "finishedAt": self._to_rfc3339(task.get("finishedAt")),
            "materialRefs": self._extract_material_refs_from_local_payload(payload),
        }
        if binding and binding.get("lease_token"):
            sync_payload["leaseToken"] = binding["lease_token"]
        return sync_payload

    def _build_remote_media_payload(self, task):
        payload = task.get("payload") or {}
        media_payload = dict(payload.get("omnidriveMediaPayload") or {})
        media_payload.setdefault("tags", payload.get("tags") or [])
        media_payload.setdefault("category", payload.get("category"))
        media_payload.setdefault("isDraft", bool(payload.get("isDraft", False)))
        media_payload.setdefault("productLink", payload.get("productLink") or "")
        media_payload.setdefault("productTitle", payload.get("productTitle") or "")
        media_payload.setdefault("publishDate", payload.get("publishDate") or 0)
        media_payload.setdefault(
            "files",
            [
                {
                    "root": payload.get("materialRoot"),
                    "path": payload.get("materialPath"),
                    "absolutePath": payload.get("sourceAbsolutePath"),
                }
            ] if payload.get("fileSourceMode") == "material" else [],
        )
        if payload.get("thumbnailSourceMode") == "material":
            media_payload.setdefault(
                "thumbnail",
                {
                    "root": payload.get("thumbnailRoot"),
                    "path": payload.get("thumbnailPath"),
                    "absolutePath": payload.get("thumbnailAbsolutePath"),
                },
            )
        if payload.get("omnidriveAICloudJobId"):
            media_payload.setdefault("aiJobId", payload.get("omnidriveAICloudJobId"))
        if payload.get("omnidriveAITaskUuid"):
            media_payload.setdefault("aiTaskUuid", payload.get("omnidriveAITaskUuid"))
        return media_payload

    def _build_remote_artifacts(self, task, verification_payload):
        artifacts = []
        artifact_path = task.get("artifactPath")
        if not artifact_path:
            return artifacts
        if verification_payload and verification_payload.get("screenshotData"):
            return artifacts
        data_url = self._artifact_file_to_data_url(artifact_path)
        if not data_url:
            return artifacts
        absolute_path = self._resolve_artifact_path(artifact_path)
        file_name = Path(absolute_path).name
        mime_type = mimetypes.guess_type(file_name)[0] or "application/octet-stream"
        artifacts.append(
            {
                "artifactKey": "local-artifact",
                "artifactType": "verification-screenshot" if mime_type.startswith("image/") else "attachment",
                "source": "agent",
                "title": "本地任务产物",
                "fileName": file_name,
                "mimeType": mime_type,
                "data": data_url,
            }
        )
        return artifacts

    def _resolve_local_account_file_path(self, platform_name, account_name):
        platform_type = PLATFORM_TYPE_BY_NAME.get(platform_name)
        if not platform_type:
            raise ValueError(f"不支持的平台: {platform_name}")

        with sqlite3.connect(self.db_path) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT filePath, status
                FROM user_info
                WHERE type = ? AND userName = ?
                ORDER BY status DESC, id DESC
                LIMIT 1
                """,
                (platform_type, account_name),
            )
            row = cursor.fetchone()
        if not row:
            raise ValueError(f"本地未找到账号: {platform_name} / {account_name}")
        return row["filePath"]

    def _load_local_accounts(self):
        with sqlite3.connect(self.db_path) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT id, type, filePath, userName, status
                FROM user_info
                ORDER BY id DESC
                """
            )
            return cursor.fetchall()

    def _extract_material_refs_from_package(self, package):
        refs = []
        for item in package.get("materials") or []:
            root_name = str(item.get("rootName") or "").strip()
            relative_path = str(item.get("relativePath") or "").strip()
            if not root_name or not relative_path:
                continue
            refs.append(
                {
                    "root": root_name,
                    "path": relative_path,
                    "role": str(item.get("role") or "media").strip() or "media",
                }
            )
        return refs

    def _extract_material_refs_from_local_payload(self, payload):
        refs = []
        existing = payload.get("omnidriveMaterialRefs") or []
        if existing:
            for item in existing:
                root_name = str(item.get("root") or "").strip()
                relative_path = str(item.get("path") or "").strip()
                if root_name and relative_path:
                    refs.append(
                        {
                            "root": root_name,
                            "path": relative_path,
                            "role": str(item.get("role") or "media").strip() or "media",
                        }
                    )
            return refs

        if payload.get("fileSourceMode") == "material" and payload.get("materialRoot") and payload.get("materialPath"):
            refs.append(
                {
                    "root": payload.get("materialRoot"),
                    "path": payload.get("materialPath"),
                    "role": "media",
                }
            )
        if payload.get("thumbnailSourceMode") == "material" and payload.get("thumbnailRoot") and payload.get("thumbnailPath"):
            refs.append(
                {
                    "root": payload.get("thumbnailRoot"),
                    "path": payload.get("thumbnailPath"),
                    "role": "thumbnail",
                }
            )
        return refs

    def _choose_primary_file(self, refs):
        media_candidates = [
            item for item in refs
            if str(item.get("role") or "media") not in {"thumbnail", "cover"}
        ]
        ordered = media_candidates or refs
        for item in ordered:
            root_name = str(item.get("root") or "").strip()
            relative_path = str(item.get("path") or "").strip()
            if not root_name or not relative_path:
                continue
            absolute_path = str(item.get("absolutePath") or "").strip()
            if not absolute_path:
                try:
                    listing = read_material_file(
                        self.material_roots,
                        root_name=root_name,
                        relative_path=relative_path,
                        max_bytes=1024,
                    )
                    absolute_path = listing["absolutePath"]
                except Exception:
                    absolute_path = ""
            if not absolute_path:
                continue
            return {
                "root": root_name,
                "path": relative_path,
                "absolutePath": absolute_path,
                "displayPath": f"{root_name}:{relative_path}",
                "name": item.get("name") or Path(absolute_path).name,
            }
        return None

    def _choose_thumbnail(self, refs, media_payload):
        thumbnail_payload = media_payload.get("thumbnail")
        if isinstance(thumbnail_payload, dict):
            root_name = str(thumbnail_payload.get("root") or "").strip()
            relative_path = str(thumbnail_payload.get("path") or "").strip()
            if root_name and relative_path:
                return {
                    "root": root_name,
                    "path": relative_path,
                    "absolutePath": thumbnail_payload.get("absolutePath"),
                }

        for item in refs:
            role = str(item.get("role") or "").strip()
            if role not in {"thumbnail", "cover"}:
                continue
            root_name = str(item.get("root") or "").strip()
            relative_path = str(item.get("path") or "").strip()
            if root_name and relative_path:
                return {
                    "root": root_name,
                    "path": relative_path,
                    "absolutePath": item.get("absolutePath"),
                }
        return None

    def _extract_publish_date(self, task, media_payload):
        value = media_payload.get("publishDate") or media_payload.get("platformPublishAt")
        if value in (None, "", 0, "0"):
            value = task.get("runAt")
        return self._normalize_datetime(value)

    @staticmethod
    def _normalize_tags(value):
        if isinstance(value, list):
            return [str(item).strip() for item in value if str(item).strip()]
        if value in (None, ""):
            return []
        return [str(value).strip()]

    def _should_sync_local_task(self, task):
        task_uuid = str(task.get("taskUuid") or "").strip()
        status = str(task.get("status") or "").strip()
        if not task_uuid or not status:
            return False
        binding = self._get_lease(task_uuid)
        if status in {"pending", "scheduled"} and binding and str(task.get("source") or "") == "omnidrive_agent":
            return False
        synced_status = binding.get("last_synced_status") if binding else None
        synced_updated_at = binding.get("last_synced_updated_at") if binding else None
        if synced_status == status and synced_updated_at == (task.get("updatedAt") or ""):
            return False
        if not binding and str(task.get("source") or "") not in SYNCABLE_LOCAL_SOURCES:
            return False
        return True

    def _mark_task_synced(self, task):
        task_uuid = task["taskUuid"]
        binding = self._get_lease(task_uuid)
        if not binding and str(task.get("source") or "") != "omnidrive_agent":
            self._upsert_lease(task_uuid, None, None)
            binding = self._get_lease(task_uuid)
        if not binding:
            return
        self._upsert_lease(
            task_uuid,
            binding.get("lease_token"),
            binding.get("lease_expires_at"),
            last_synced_status=task.get("status"),
            last_synced_updated_at=task.get("updatedAt"),
        )

    @staticmethod
    def _map_local_status(status):
        if status in {"pending", "running", "success", "failed", "needs_verify", "cancelled"}:
            return status
        if status == "scheduled":
            return "pending"
        return None

    @staticmethod
    def _runtime_stage_for_status(status):
        return {
            "pending": "queued_local",
            "scheduled": "queued_local",
            "running": "running",
            "success": "completed",
            "failed": "failed",
            "needs_verify": "needs_verify",
            "cancelled": "cancelled",
        }.get(status, status)

    def _build_sync_ai_task_payload(self, task):
        payload = task.get("payload") or {}
        return {
            "id": task["taskUuid"],
            "deviceCode": self.device_code,
            "skillId": payload.get("skillId"),
            "jobType": task.get("jobType"),
            "modelName": task.get("modelName"),
            "prompt": task.get("prompt"),
            "inputPayload": payload.get("inputPayload") or {},
            "publishPayload": payload.get("publishPayload") or {},
            "status": task.get("status"),
            "message": task.get("message"),
            "runAt": self._to_rfc3339(payload.get("runAt")),
        }

    def _download_ai_artifacts(self, local_task, artifacts):
        task_uuid = str(local_task.get("taskUuid") or "").strip()
        if not task_uuid:
            return []
        target_dir = self.generated_root_path / task_uuid
        target_dir.mkdir(parents=True, exist_ok=True)

        refs = []
        for index, artifact in enumerate(artifacts):
            public_url = str(artifact.get("publicUrl") or "").strip()
            mime_type = str(artifact.get("mimeType") or "").strip()
            artifact_type = str(artifact.get("artifactType") or "").strip()
            file_name = str(artifact.get("fileName") or f"artifact-{index + 1}").strip() or f"artifact-{index + 1}"
            if not public_url:
                continue
            if public_url.startswith("/"):
                public_url = f"{self.cloud_base_url}{public_url}"
            if mime_type and not (mime_type.startswith("image/") or mime_type.startswith("video/")):
                continue

            response = self._session.get(public_url, timeout=self.http_timeout)
            response.raise_for_status()
            target_path = target_dir / file_name
            target_path.write_bytes(response.content)
            relative_path = target_path.relative_to(self.generated_root_path).as_posix()
            role = "thumbnail" if artifact_type in {"thumbnail", "cover"} else "media"
            refs.append(
                {
                    "root": self.generated_root_name,
                    "path": relative_path,
                    "absolutePath": str(target_path),
                    "name": file_name,
                    "role": role,
                    "artifactKey": artifact.get("artifactKey"),
                    "mimeType": mime_type or mimetypes.guess_type(file_name)[0],
                }
            )
        return refs

    def _enqueue_publish_from_ai_task(self, local_task, artifact_refs):
        payload = local_task.get("payload") or {}
        publish_payload = dict(payload.get("publishPayload") or {})
        targets = self._resolve_ai_publish_targets(publish_payload)
        if not targets:
            return None

        primary_file = next((item for item in artifact_refs if item.get("role") != "thumbnail"), None)
        if not primary_file:
            return None
        thumbnail_ref = next((item for item in artifact_refs if item.get("role") == "thumbnail"), None)
        title = str(publish_payload.get("title") or local_task.get("prompt") or "AI 生成内容").strip() or "AI 生成内容"
        run_at = self._normalize_datetime(publish_payload.get("runAt") or payload.get("runAt"))
        status = "scheduled" if self._is_future_datetime(run_at) else "pending"
        publish_date = self._normalize_datetime(
            publish_payload.get("publishDate") or publish_payload.get("requestedRun") or 0
        )

        material_refs = [
            {"root": item["root"], "path": item["path"], "role": item.get("role") or "media"}
            for item in artifact_refs
        ]
        specs = []
        for target in targets:
            local_publish_task_uuid = str(uuid.uuid4())
            local_spec = {
                "taskUuid": local_publish_task_uuid,
                "source": "omnidrive_ai",
                "platformType": target["platformType"],
                "platformName": target["platformName"],
                "accountName": target["accountName"],
                "accountFilePath": target["accountFilePath"],
                "fileName": primary_file["name"],
                "filePath": f"{primary_file['root']}:{primary_file['path']}",
                "title": title,
                "runAt": run_at,
                "platformPublishAt": publish_date,
                "status": status,
                "message": "等待 AI 产物发布" if status == "pending" else "等待 AI 产物定时发布",
                "payload": {
                    "platformType": target["platformType"],
                    "platformName": target["platformName"],
                    "title": title,
                    "tags": publish_payload.get("tags") or [],
                    "filePath": f"{primary_file['root']}:{primary_file['path']}",
                    "accountFilePath": target["accountFilePath"],
                    "accountName": target["accountName"],
                    "publishDate": publish_date or 0,
                    "category": publish_payload.get("category"),
                    "fileSourceMode": "material",
                    "materialRoot": primary_file["root"],
                    "materialPath": primary_file["path"],
                    "sourceAbsolutePath": primary_file["absolutePath"],
                    "productLink": publish_payload.get("productLink") or "",
                    "productTitle": publish_payload.get("productTitle") or "",
                    "isDraft": bool(publish_payload.get("isDraft", False)),
                    "contentText": publish_payload.get("contentText"),
                    "omnidriveAICloudJobId": local_task.get("cloudJobId"),
                    "omnidriveAITaskUuid": local_task.get("taskUuid"),
                    "omnidriveMaterialRefs": material_refs,
                },
            }
            if thumbnail_ref:
                local_spec["payload"].update(
                    {
                        "thumbnailSourceMode": "material",
                        "thumbnailRoot": thumbnail_ref["root"],
                        "thumbnailPath": thumbnail_ref["path"],
                        "thumbnailAbsolutePath": thumbnail_ref["absolutePath"],
                    }
                )
            specs.append(local_spec)

        if not specs:
            return None

        self.publish_task_manager.enqueue_specs(specs)
        return specs[0]["taskUuid"]

    @staticmethod
    def _local_ai_status_from_remote_job(job):
        job_status = str(job.get("status") or "").strip()
        delivery_status = str(job.get("deliveryStatus") or "").strip()
        if delivery_status in {"publish_queued", "publishing", "success", "needs_verify", "failed", "cancelled"}:
            return {
                "publish_queued": "publish_pending",
                "publishing": "publishing",
                "success": "success",
                "needs_verify": "needs_verify",
                "failed": "failed",
                "cancelled": "cancelled",
            }[delivery_status]
        if job_status == "scheduled":
            return "scheduled"
        if job_status in {"queued", "pending"}:
            return "queued_cloud"
        if job_status == "running":
            return "generating"
        if job_status in {"success", "completed"}:
            return "output_ready"
        if job_status == "failed":
            return "failed"
        if job_status == "cancelled":
            return "cancelled"
        return "queued_cloud"

    def _resolve_ai_publish_targets(self, publish_payload):
        targets = []
        seen = set()
        raw_targets = publish_payload.get("targets") or []
        if isinstance(raw_targets, dict):
            raw_targets = [raw_targets]

        if isinstance(raw_targets, list):
            for item in raw_targets:
                if not isinstance(item, dict):
                    continue
                resolved = self._resolve_ai_publish_target(
                    platform_value=item.get("platform") or item.get("platformName"),
                    platform_type_value=item.get("platformType") or item.get("type"),
                    account_name=item.get("accountName"),
                    account_file_path=item.get("accountFilePath"),
                )
                if not resolved:
                    continue
                key = (
                    resolved["platformType"],
                    resolved["platformName"],
                    resolved["accountName"],
                    resolved["accountFilePath"],
                )
                if key in seen:
                    continue
                seen.add(key)
                targets.append(resolved)

        if targets:
            return targets

        resolved = self._resolve_ai_publish_target(
            platform_value=publish_payload.get("platform") or publish_payload.get("platformName"),
            platform_type_value=publish_payload.get("platformType") or publish_payload.get("type"),
            account_name=publish_payload.get("accountName"),
            account_file_path=publish_payload.get("accountFilePath"),
        )
        return [resolved] if resolved else []

    def _resolve_ai_publish_target(
        self,
        *,
        platform_value=None,
        platform_type_value=None,
        account_name=None,
        account_file_path=None,
    ):
        account_name = str(account_name or "").strip()
        if not account_name:
            return None

        platform_type = 0
        platform_name = ""
        if platform_type_value not in (None, ""):
            try:
                platform_type = int(platform_type_value)
            except (TypeError, ValueError):
                platform_type = 0
            platform_name = PLATFORM_NAME_BY_TYPE.get(platform_type) or ""

        raw_platform = str(platform_value or "").strip()
        if raw_platform and not platform_name:
            if raw_platform.isdigit():
                platform_type = int(raw_platform)
                platform_name = PLATFORM_NAME_BY_TYPE.get(platform_type) or ""
            else:
                target = LOGIN_PLATFORM_ALIAS_MAP.get(raw_platform.lower())
                if target:
                    platform_type = int(target["type"])
                    platform_name = str(target["label"]).strip()

        if not platform_type or not platform_name:
            agent_logger.warning(
                "omnidrive bridge skipped ai publish target due to unsupported platform platform={} account_name={}",
                platform_value,
                account_name,
            )
            return None

        account_file_path = str(account_file_path or "").strip()
        if not account_file_path:
            try:
                account_file_path = self._resolve_local_account_file_path(platform_name, account_name)
            except Exception as exc:
                agent_logger.warning(
                    "omnidrive bridge skipped ai publish target because local account was not found platform={} account_name={} error={}",
                    platform_name,
                    account_name,
                    exc,
                )
                return None

        return {
            "platformType": platform_type,
            "platformName": platform_name,
            "accountName": account_name,
            "accountFilePath": account_file_path,
        }

    @staticmethod
    def _map_publish_status_to_local_ai_status(publish_status):
        return {
            "pending": "publish_pending",
            "scheduled": "publish_pending",
            "running": "publishing",
            "success": "success",
            "needs_verify": "needs_verify",
            "failed": "failed",
            "cancelled": "cancelled",
        }.get(str(publish_status or "").strip())

    @staticmethod
    def _map_publish_status_to_delivery_status(publish_status):
        value = str(publish_status or "").strip()
        if value in {"pending", "scheduled"}:
            return "publish_queued"
        if value == "running":
            return "publishing"
        if value in {"success", "needs_verify", "failed", "cancelled"}:
            return value
        return "imported"

    def _count_inflight_omnidrive_tasks(self):
        tasks = self.publish_task_manager.list_tasks(limit=500, sources=["omnidrive_agent"])
        return sum(1 for task in tasks if task.get("status") in {"pending", "scheduled", "running"})

    def _build_runtime_payload(self):
        task_counts = self.publish_task_manager.list_tasks(limit=500)
        by_status = {}
        by_source = {}
        for task in task_counts:
            status = str(task.get("status") or "").strip()
            source = str(task.get("source") or "local_api").strip() or "local_api"
            by_status[status] = by_status.get(status, 0) + 1
            by_source[source] = by_source.get(source, 0) + 1
        active_leases = [
            binding for binding in self._list_leases()
            if str(binding.get("lease_token") or "").strip()
        ]
        login_worker = self._get_active_login_worker()
        return {
            "publishTasks": by_status,
            "publishTasksBySource": by_source,
            "aiTasks": self.ai_task_manager.summary() if self.ai_task_manager else {},
            "materialRoots": len(self.material_roots),
            "activeLeaseCount": len(active_leases),
            "activeLeaseTaskIds": [binding.get("task_uuid") for binding in active_leases[:20]],
            "activeLoginSessionId": login_worker.get("sessionId") if login_worker else None,
            "activeLoginPlatform": login_worker.get("platform") if login_worker else None,
            "activeLoginAccountName": login_worker.get("accountName") if login_worker else None,
        }

    def _build_status_snapshot(self):
        tasks = self.publish_task_manager.list_tasks(limit=500, sources=SYNCABLE_LOCAL_SOURCES)
        by_source = {}
        by_status = {}
        imported_queue = 0
        local_origin = 0
        for task in tasks:
            source = str(task.get("source") or "local_api").strip() or "local_api"
            status = str(task.get("status") or "").strip()
            by_source[source] = by_source.get(source, 0) + 1
            by_status[status] = by_status.get(status, 0) + 1
            if source == "omnidrive_agent":
                imported_queue += 1
            else:
                local_origin += 1

        leases = self._list_leases()
        active_leases = []
        for binding in leases:
            lease_token = str(binding.get("lease_token") or "").strip()
            if not lease_token:
                continue
            active_leases.append(
                {
                    "taskUuid": binding.get("task_uuid"),
                    "leaseExpiresAt": binding.get("lease_expires_at"),
                    "lastSyncedStatus": binding.get("last_synced_status"),
                    "lastSyncedUpdatedAt": binding.get("last_synced_updated_at"),
                }
            )

        login_worker = self._get_active_login_worker()
        login_snapshot = None
        if login_worker:
            login_snapshot = {
                "sessionId": login_worker.get("sessionId"),
                "platform": login_worker.get("platform"),
                "platformLabel": login_worker.get("platformLabel"),
                "accountName": login_worker.get("accountName"),
                "status": login_worker.get("lastStatus"),
                "message": login_worker.get("lastMessage"),
                "startedAt": login_worker.get("startedAt"),
                "isAlive": bool(login_worker.get("thread") and login_worker["thread"].is_alive()),
            }

        return {
            "bridgeTaskCountsBySource": by_source,
            "bridgeTaskCountsByStatus": by_status,
            "aiTaskSummary": self.ai_task_manager.summary() if self.ai_task_manager else {},
            "activeLeaseCount": len(active_leases),
            "activeLeases": active_leases[:20],
            "importedCloudQueueCount": imported_queue,
            "mirroredLocalTaskCount": local_origin,
            "skillCacheCount": self._count_cached_skills(),
            "activeLoginWorker": login_snapshot,
        }

    def _count_cached_skills(self):
        if not self._skill_cache_dir.exists():
            return 0
        return sum(1 for item in self._skill_cache_dir.iterdir() if item.is_dir())

    @staticmethod
    def _load_cached_skill_manifest(manifest_path):
        path = Path(manifest_path)
        if not path.exists() or not path.is_file():
            return None
        try:
            with open(path, "r", encoding="utf-8") as manifest_file:
                return json.load(manifest_file)
        except (OSError, json.JSONDecodeError):
            return None

    def _build_cached_skill_snapshot(self, manifest, manifest_path, include_assets=False):
        skill = manifest.get("skill") or {}
        sync = manifest.get("sync") or {}
        assets = manifest.get("assets") or []
        revision = str(manifest.get("revision") or "").strip()
        synced_revision = str(sync.get("syncedRevision") or "").strip() or None
        desired_revision = str(sync.get("desiredRevision") or revision or "").strip() or None
        sync_status = str(sync.get("syncStatus") or "").strip() or None
        is_current = sync.get("isCurrent")
        if is_current is None:
            is_current = bool(synced_revision and desired_revision and synced_revision == desired_revision)
        needs_sync = sync.get("needsSync")
        if needs_sync is None:
            needs_sync = bool(desired_revision and synced_revision and desired_revision != synced_revision)

        asset_items = [self._build_cached_skill_asset_snapshot(asset) for asset in assets]
        item = {
            "skillId": str(skill.get("id") or manifest_path.parent.name).strip(),
            "revision": revision or None,
            "name": str(skill.get("name") or "").strip() or None,
            "description": skill.get("description"),
            "outputType": str(skill.get("outputType") or "").strip() or None,
            "isEnabled": bool(skill.get("isEnabled", True)),
            "manifestPath": str(manifest_path),
            "assetsDir": str(manifest_path.parent / "assets"),
            "assetCount": len(asset_items),
            "assetTypes": sorted({str(asset.get("assetType") or "").strip() for asset in assets if str(asset.get("assetType") or "").strip()}),
            "syncStatus": sync_status,
            "syncedRevision": synced_revision,
            "desiredRevision": desired_revision,
            "isCurrent": is_current,
            "needsSync": needs_sync,
            "lastSyncedAt": sync.get("lastSyncedAt") or (manifest.get("cache") or {}).get("syncedAt"),
            "syncMessage": sync.get("message"),
        }
        if include_assets:
            item["assets"] = asset_items
            item["manifest"] = manifest
        return item

    @staticmethod
    def _build_cached_skill_asset_snapshot(asset):
        local_path = str(asset.get("localPath") or "").strip()
        metadata_path = str(asset.get("metadataPath") or "").strip()
        return {
            "id": asset.get("id"),
            "assetType": asset.get("assetType"),
            "fileName": asset.get("fileName"),
            "mimeType": asset.get("mimeType"),
            "publicUrl": asset.get("publicUrl"),
            "sizeBytes": asset.get("sizeBytes"),
            "localFileName": asset.get("localFileName"),
            "localPath": local_path or None,
            "metadataPath": metadata_path or None,
            "downloadStatus": asset.get("downloadStatus"),
            "downloadError": asset.get("downloadError"),
            "exists": Path(local_path).exists() if local_path else False,
            "metadataExists": Path(metadata_path).exists() if metadata_path else False,
        }

    def _normalize_cloud_public_url(self, public_url):
        value = str(public_url or "").strip()
        if not value:
            return ""
        if value.startswith("/"):
            return f"{self.cloud_base_url}{value}"
        return value

    @staticmethod
    def _allocate_skill_asset_file_name(asset, used_names):
        original_name = str(asset.get("fileName") or asset.get("id") or "asset.bin").strip() or "asset.bin"
        candidate = original_name
        asset_id = str(asset.get("id") or "").strip()
        if candidate in used_names and asset_id:
            suffix = Path(original_name).suffix
            stem = Path(original_name).stem or "asset"
            candidate = f"{stem}-{asset_id[:8]}{suffix}"
        sequence = 2
        while candidate in used_names:
            suffix = Path(original_name).suffix
            stem = Path(original_name).stem or "asset"
            candidate = f"{stem}-{sequence}{suffix}"
            sequence += 1
        used_names.add(candidate)
        return candidate

    @staticmethod
    def _cleanup_skill_asset_dir(assets_dir, expected_names):
        path = Path(assets_dir)
        if not path.exists():
            return
        for item in path.iterdir():
            if item.is_dir():
                shutil.rmtree(item, ignore_errors=True)
                continue
            if item.name in expected_names:
                continue
            item.unlink(missing_ok=True)

    @staticmethod
    def _normalize_publish_task_queue(payload):
        if isinstance(payload, dict):
            items = payload.get("readyItems")
            if isinstance(items, list):
                return items
            return []
        if isinstance(payload, list):
            return payload
        return []

    @staticmethod
    def _format_remote_error(exc):
        response = getattr(exc, "response", None)
        if response is None:
            return str(exc)
        message = OmniDriveBridge._extract_remote_error_message(response)
        if message:
            return f"{response.status_code} {message}"
        text = str(getattr(response, "text", "") or "").strip()
        if text:
            return f"{response.status_code} {text[:300]}"
        return f"{response.status_code} {exc}"

    @staticmethod
    def _extract_remote_error_message(response):
        if response is None:
            return ""
        try:
            payload = response.json()
        except ValueError:
            payload = None
        if isinstance(payload, dict):
            message = str(payload.get("error") or payload.get("message") or "").strip()
            if message:
                return message
        return ""

    def _artifact_file_to_data_url(self, artifact_path):
        absolute_path = self._resolve_artifact_path(artifact_path)
        if not absolute_path.exists() or not absolute_path.is_file():
            return None
        mime_type = mimetypes.guess_type(absolute_path.name)[0] or "application/octet-stream"
        encoded = base64.b64encode(absolute_path.read_bytes()).decode("utf-8")
        return f"data:{mime_type};base64,{encoded}"

    @staticmethod
    def _resolve_artifact_path(artifact_path):
        path = Path(str(artifact_path))
        if path.is_absolute():
            return path
        return Path(BASE_DIR / path)

    def _init_db(self):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS omnidrive_task_leases (
                    task_uuid TEXT PRIMARY KEY,
                    lease_token TEXT,
                    lease_expires_at TEXT,
                    last_synced_status TEXT,
                    last_synced_updated_at TEXT,
                    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
                )
                """
            )
            conn.commit()

    def _get_lease(self, task_uuid):
        with sqlite3.connect(self.db_path) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT task_uuid, lease_token, lease_expires_at, last_synced_status, last_synced_updated_at
                FROM omnidrive_task_leases
                WHERE task_uuid = ?
                """,
                (task_uuid,),
            )
            row = cursor.fetchone()
        return dict(row) if row else None

    def _list_leases(self):
        with sqlite3.connect(self.db_path) as conn:
            conn.row_factory = sqlite3.Row
            cursor = conn.cursor()
            cursor.execute(
                """
                SELECT task_uuid, lease_token, lease_expires_at, last_synced_status, last_synced_updated_at
                FROM omnidrive_task_leases
                """
            )
            rows = cursor.fetchall()
        return [dict(row) for row in rows]

    def _upsert_lease(
        self,
        task_uuid,
        lease_token,
        lease_expires_at,
        last_synced_status=None,
        last_synced_updated_at=None,
    ):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO omnidrive_task_leases (
                    task_uuid, lease_token, lease_expires_at, last_synced_status, last_synced_updated_at
                )
                VALUES (?, ?, ?, ?, ?)
                ON CONFLICT(task_uuid) DO UPDATE
                SET lease_token = COALESCE(excluded.lease_token, omnidrive_task_leases.lease_token),
                    lease_expires_at = COALESCE(excluded.lease_expires_at, omnidrive_task_leases.lease_expires_at),
                    last_synced_status = COALESCE(excluded.last_synced_status, omnidrive_task_leases.last_synced_status),
                    last_synced_updated_at = COALESCE(excluded.last_synced_updated_at, omnidrive_task_leases.last_synced_updated_at),
                    updated_at = CURRENT_TIMESTAMP
                """,
                (task_uuid, lease_token, lease_expires_at, last_synced_status, last_synced_updated_at),
            )
            conn.commit()

    def _delete_lease(self, task_uuid):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM omnidrive_task_leases WHERE task_uuid = ?", (task_uuid,))
            conn.commit()

    def _clear_lease_token(self, task_uuid):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                UPDATE omnidrive_task_leases
                SET lease_token = NULL,
                    lease_expires_at = NULL,
                    updated_at = CURRENT_TIMESTAMP
                WHERE task_uuid = ?
                """,
                (task_uuid,),
            )
            conn.commit()

    def _finalize_lease_record(self, task_uuid, status, updated_at):
        with sqlite3.connect(self.db_path) as conn:
            cursor = conn.cursor()
            cursor.execute(
                """
                INSERT INTO omnidrive_task_leases (
                    task_uuid, lease_token, lease_expires_at, last_synced_status, last_synced_updated_at
                )
                VALUES (?, NULL, NULL, ?, ?)
                ON CONFLICT(task_uuid) DO UPDATE
                SET lease_token = NULL,
                    lease_expires_at = NULL,
                    last_synced_status = excluded.last_synced_status,
                    last_synced_updated_at = excluded.last_synced_updated_at,
                    updated_at = CURRENT_TIMESTAMP
                """,
                (task_uuid, status, updated_at),
            )
            conn.commit()

    @staticmethod
    def _lease_needs_renew(value):
        if not value:
            return True
        try:
            expires_at = datetime.fromisoformat(str(value).replace("Z", "+00:00"))
        except ValueError:
            return True
        return (expires_at.timestamp() - time.time()) < 20

    @staticmethod
    def _local_timezone():
        return datetime.now().astimezone().tzinfo or timezone.utc

    @classmethod
    def _parse_datetime_value(cls, value):
        if value in (None, "", 0, "0"):
            return None
        if isinstance(value, datetime):
            return value

        value_str = str(value).strip()
        if not value_str:
            return None

        try:
            iso_parsed = datetime.fromisoformat(value_str.replace("Z", "+00:00"))
            if (
                iso_parsed.tzinfo is not None
                or "T" in value_str
                or value_str.endswith("Z")
                or "+" in value_str[10:]
                or "-" in value_str[10:]
            ):
                return iso_parsed
        except ValueError:
            pass

        normalized = value_str.replace("T", " ")
        if normalized.endswith("Z"):
            normalized = normalized[:-1]
        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%d %H:%M", "%Y-%m-%d %H:%M:%S.%f"):
            try:
                return datetime.strptime(normalized, fmt)
            except ValueError:
                continue

        try:
            return datetime.fromisoformat(value_str.replace("Z", "+00:00"))
        except ValueError:
            return None

    @classmethod
    def _normalize_datetime(cls, value):
        if value in (None, "", 0, "0"):
            return None
        parsed = cls._parse_datetime_value(value)
        if parsed is not None:
            if parsed.tzinfo is not None:
                parsed = parsed.astimezone(cls._local_timezone()).replace(tzinfo=None)
            return parsed.strftime("%Y-%m-%d %H:%M:%S")

        value_str = str(value).strip().replace("T", " ")
        return value_str[:-1] if value_str.endswith("Z") else value_str

    @classmethod
    def _is_future_datetime(cls, value):
        if not value:
            return False
        parsed = cls._parse_datetime_value(value)
        if parsed is None:
            return False
        if parsed.tzinfo is not None:
            parsed = parsed.astimezone(cls._local_timezone()).replace(tzinfo=None)
        return parsed > datetime.now()

    @classmethod
    def _to_rfc3339(cls, value):
        if value in (None, "", 0, "0"):
            return None
        parsed = cls._parse_datetime_value(value)
        if parsed is None:
            return None
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=cls._local_timezone())
        return parsed.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")

    @staticmethod
    def _now_string():
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")

    @staticmethod
    def _iso_now():
        return datetime.utcnow().isoformat() + "Z"
