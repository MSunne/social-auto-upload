#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
RUNTIME_DIR="${ROOT_DIR}/runtime"
PID_FILE="${RUNTIME_DIR}/cloud_live_sync.pid"
LOG_FILE="${RUNTIME_DIR}/cloud_live_sync.log"
SESSION_NAME="cloud_live_sync"

mkdir -p "${RUNTIME_DIR}"
screen -wipe >/dev/null 2>&1 || true

if ps -ef | grep "[c]loud_live_sync.py --skip-initial-sync" >/dev/null 2>&1; then
  echo "cloud live sync already running session=${SESSION_NAME}"
  exit 0
fi

screen -dmS "${SESSION_NAME}" bash -lc "cd '${ROOT_DIR}' && exec python3 '${SCRIPT_DIR}/cloud_live_sync.py' --skip-initial-sync >>'${LOG_FILE}' 2>&1"
echo "${SESSION_NAME}" >"${PID_FILE}"
echo "cloud live sync started session=${SESSION_NAME} log=${LOG_FILE}"
