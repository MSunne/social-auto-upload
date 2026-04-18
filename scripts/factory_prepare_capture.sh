#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

APP_ROOT="${OMNIBULL_APP_ROOT:-/opt/omnibull/social-auto-upload}"
APP_USER="${OMNIBULL_APP_USER:-sun}"
APP_GROUP="${OMNIBULL_APP_GROUP:-${APP_USER}}"
PERSISTENT_ROOT="${OMNIBULL_PERSISTENT_ROOT:-/persistent/omnibull}"
RUNTIME_DIR="${OMNIBULL_RUNTIME_DIR:-${PERSISTENT_ROOT}/runtime}"
WORKSPACE_DIR="${OMNIBULL_WORKSPACE_DIR:-${PERSISTENT_ROOT}/workspace}"
DB_PATH="${OMNIBULL_DB_PATH:-${RUNTIME_DIR}/database.db}"
RESTART_SERVICE="${OMNIBULL_FACTORY_RESTART_AFTER_PREP:-0}"
STOP_SERVICES_RAW="${OMNIBULL_FACTORY_STOP_SERVICES:-sau-stack.service}"

declare -a STOPPED_SERVICES=()


log() {
  printf '[%s] [factory-prepare] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}


trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s\n' "${value}"
}


stop_services() {
  local raw item state
  IFS=',' read -r -a raw <<< "${STOP_SERVICES_RAW}"
  for item in "${raw[@]}"; do
    item="$(trim "${item}")"
    [[ -n "${item}" ]] || continue
    state="$(systemctl is-active "${item}" 2>/dev/null || true)"
    if [[ -n "${state}" && "${state}" != "inactive" && "${state}" != "failed" ]]; then
      log "stopping service ${item}"
      systemctl stop "${item}"
      STOPPED_SERVICES+=("${item}")
    fi
  done
}


restart_services() {
  local item
  [[ "${RESTART_SERVICE}" == "1" ]] || return 0
  if [[ "${#STOPPED_SERVICES[@]}" -eq 0 ]]; then
    return 0
  fi
  for item in "${STOPPED_SERVICES[@]}"; do
    log "starting service ${item}"
    systemctl start "${item}" || true
  done
}


clear_device_identity() {
  log "removing cloned device identity"
  rm -f /etc/omnibull/device.json
  rm -f "${APP_ROOT}/runtime/device.identity.json"
  rm -f "${RUNTIME_DIR}/device.identity.json"
  find /home -type f \( \
    -path '*/.local/state/omnibull/device.json' -o \
    -path '*/Library/Application Support/OmniBull/device.json' \
  \) -delete 2>/dev/null || true
}


clear_runtime_files() {
  log "clearing logs, pids, and task artifacts"
  find "${RUNTIME_DIR}" -maxdepth 1 -type f \( -name '*.pid' -o -name '*.tmp' \) -delete 2>/dev/null || true
  rm -rf "${RUNTIME_DIR}/taskArtifacts" "${RUNTIME_DIR}/cookiesFile"
  mkdir -p "${RUNTIME_DIR}/taskArtifacts" "${RUNTIME_DIR}/cookiesFile"
  rm -rf "${PERSISTENT_ROOT}/logs"
  mkdir -p "${PERSISTENT_ROOT}/logs"
}


clear_workspace_files() {
  log "removing uploaded materials and generated workspace data"
  rm -rf "${WORKSPACE_DIR}"
  mkdir -p \
    "${WORKSPACE_DIR}" \
    "${WORKSPACE_DIR}/videoFile" \
    "${WORKSPACE_DIR}/omnidriveSync" \
    "${WORKSPACE_DIR}/omnidriveSync/generated"
}


reset_local_database() {
  [[ -f "${DB_PATH}" ]] || return 0
  log "clearing local SQLite business data"
  python3 - "${DB_PATH}" <<'PY'
import sqlite3
import sys

db_path = sys.argv[1]
conn = sqlite3.connect(db_path)
cur = conn.cursor()
for statement in (
    "DELETE FROM user_info",
    "DELETE FROM file_records",
    "DELETE FROM publish_tasks",
    "DELETE FROM omnidrive_ai_tasks",
):
    try:
        cur.execute(statement)
    except sqlite3.OperationalError:
        pass
conn.commit()
conn.close()
PY
}


normalize_persistent_permissions() {
  log "normalizing ownership for mutable OmniBull state"
  if [[ -d "${PERSISTENT_ROOT}" ]]; then
    chown -R "${APP_USER}:${APP_GROUP}" "${PERSISTENT_ROOT}"
    find "${PERSISTENT_ROOT}" -type d -exec chmod 0755 {} +
  fi
}


main() {
  stop_services
  clear_device_identity
  clear_runtime_files
  clear_workspace_files
  reset_local_database
  normalize_persistent_permissions
  restart_services
  log "factory capture preparation completed"
}


main "$@"
