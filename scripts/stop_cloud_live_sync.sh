#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PID_FILE="${ROOT_DIR}/runtime/cloud_live_sync.pid"
SESSION_NAME="cloud_live_sync"

screen -wipe >/dev/null 2>&1 || true
SESSIONS="$(screen -ls | awk "/[.]${SESSION_NAME}[[:space:]]/ { print \$1 }")"

if [[ -n "${SESSIONS}" ]]; then
  while IFS= read -r SESSION; do
    [[ -n "${SESSION}" ]] || continue
    screen -S "${SESSION}" -X quit || true
  done <<< "${SESSIONS}"
  echo "cloud live sync stopped session=${SESSION_NAME}"
else
  echo "cloud live sync is not running"
fi

pkill -f "[c]loud_live_sync.py --skip-initial-sync" >/dev/null 2>&1 || true

rm -f "${PID_FILE}"
