#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SSH_KEY_PATH="${OMNIDRIVE_LIVE_SYNC_SSH_KEY:-$HOME/.ssh/omnidrive_live_sync}"
REMOTE_HOST="${OMNIDRIVE_LIVE_SYNC_REMOTE_HOST:-root@43.98.251.225}"
LOCAL_SEED_SCRIPT="${ROOT_DIR}/scripts/seed_omnidrive_mock_data.py"
REMOTE_SEED_SCRIPT="/tmp/seed_omnidrive_mock_data.py"
REMOTE_SEED_ROOT="${OMNIDRIVE_REMOTE_DEMO_SEED_ROOT:-/www/wwwroot/OmniDriveCloud/data/mock-seed}"
REMOTE_BASE_URL="${OMNIDRIVE_REMOTE_BASE_URL:-http://127.0.0.1:8410}"

if [[ ! -f "${LOCAL_SEED_SCRIPT}" ]]; then
  echo "seed script not found: ${LOCAL_SEED_SCRIPT}" >&2
  exit 1
fi

scp -i "${SSH_KEY_PATH}" -o BatchMode=yes "${LOCAL_SEED_SCRIPT}" "${REMOTE_HOST}:${REMOTE_SEED_SCRIPT}"
ssh -i "${SSH_KEY_PATH}" -o BatchMode=yes "${REMOTE_HOST}" \
  "set -e; mkdir -p '${REMOTE_SEED_ROOT}'; python3 '${REMOTE_SEED_SCRIPT}' --base-url '${REMOTE_BASE_URL}' --seed-root '${REMOTE_SEED_ROOT}'"
