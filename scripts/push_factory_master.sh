#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

REMOTE_HOST="${OMNIBULL_FACTORY_HOST:-${1:-}}"
REMOTE_USER="${OMNIBULL_FACTORY_USER:-${2:-sun}}"
REMOTE_APP_ROOT="${OMNIBULL_FACTORY_APP_ROOT:-/opt/omnibull/social-auto-upload}"
REMOTE_TMP_DIR="${OMNIBULL_FACTORY_TMP_DIR:-/tmp/omnibull-factory-sync}"
OMNIDRIVE_BASE_URL="${OMNIDRIVE_BASE_URL:-}"


usage() {
  cat <<EOF
Usage:
  $(basename "$0") <host> [user]

Examples:
  $(basename "$0") 192.168.1.13 sun

Environment overrides:
  OMNIBULL_FACTORY_HOST        Remote host or IP.
  OMNIBULL_FACTORY_USER        Remote SSH user (default: sun).
  OMNIBULL_FACTORY_APP_ROOT    Remote code directory (default: /opt/omnibull/social-auto-upload).
  OMNIBULL_FACTORY_TMP_DIR     Remote temp extraction directory.
  OMNIDRIVE_BASE_URL           OmniDrive cloud URL passed into setup_factory_master.sh.

Notes:
  - This script pushes the current workspace to the mother machine without using git.
  - SSH and sudo may still prompt for the remote account password.
EOF
}


require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "missing required command: ${cmd}" >&2
    exit 1
  fi
}


prepare_remote_tree() {
  ssh -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p '${REMOTE_TMP_DIR}' '${REMOTE_APP_ROOT}'"
}


push_archive() {
  COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar \
    --exclude='.git' \
    --exclude='.venv' \
    --exclude='node_modules' \
    --exclude='sau_frontend/node_modules' \
    --exclude='OmniDriveAdmin/node_modules' \
    --exclude='omnidrive_frontend/node_modules' \
    --exclude='logs' \
    --exclude='runtime' \
    --exclude='videoFile' \
    --exclude='cookiesFile' \
    --exclude='taskArtifacts' \
    --exclude='omnidriveSync' \
    --exclude='conf.py' \
    -C "${ROOT_DIR}" \
    -czf - . \
    | ssh -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}" \
      "rm -rf '${REMOTE_TMP_DIR}/src' && mkdir -p '${REMOTE_TMP_DIR}/src' && tar -xzf - -C '${REMOTE_TMP_DIR}/src'"
}


install_remote_tree() {
  ssh -t -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}" "
    set -e
    find '${REMOTE_APP_ROOT}' -mindepth 1 -maxdepth 1 \
      ! -name 'conf.py' \
      ! -name 'logs' \
      ! -name 'runtime' \
      ! -name 'videoFile' \
      ! -name 'cookiesFile' \
      ! -name 'taskArtifacts' \
      ! -name 'omnidriveSync' \
      -exec rm -rf {} +
    cp -a '${REMOTE_TMP_DIR}/src/.' '${REMOTE_APP_ROOT}/'
  "
}


run_remote_setup() {
  local setup_cmd
  setup_cmd="cd '${REMOTE_APP_ROOT}' && sudo -E OMNIDRIVE_BASE_URL='${OMNIDRIVE_BASE_URL}' bash '${REMOTE_APP_ROOT}/scripts/setup_factory_master.sh'"
  ssh -t -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}" "${setup_cmd}"
}


main() {
  require_cmd ssh
  require_cmd tar

  if [[ -z "${REMOTE_HOST}" ]]; then
    usage
    exit 1
  fi

  prepare_remote_tree
  push_archive
  install_remote_tree
  run_remote_setup
}


main "$@"
