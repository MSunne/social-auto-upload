#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

REMOTE_HOST="${OMNIBULL_FACTORY_HOST:-${1:-}}"
REMOTE_USER="${OMNIBULL_FACTORY_USER:-${2:-sun}}"
REMOTE_SSH_PORT="${OMNIBULL_FACTORY_SSH_PORT:-22}"
REMOTE_APP_ROOT="${OMNIBULL_FACTORY_APP_ROOT:-/opt/omnibull/social-auto-upload}"
REMOTE_TMP_DIR="${OMNIBULL_FACTORY_TMP_DIR:-/tmp/omnibull-factory-sync}"
OMNIDRIVE_BASE_URL="${OMNIDRIVE_BASE_URL:-https://aitoplus.com}"
LOCAL_OPENCLAW_HOME="${OMNIBULL_LOCAL_OPENCLAW_HOME:-${HOME}/.openclaw}"
LOCAL_OPENCLAW_BUNDLE_DIR=""


usage() {
  cat <<EOF
Usage:
  $(basename "$0") <host> [user]

Examples:
  $(basename "$0") 192.168.1.13 sun

Environment overrides:
  OMNIBULL_FACTORY_HOST        Remote host or IP.
  OMNIBULL_FACTORY_USER        Remote SSH user (default: sun).
  OMNIBULL_FACTORY_SSH_PORT    Remote SSH port (default: 22).
  OMNIBULL_FACTORY_APP_ROOT    Remote code directory (default: /opt/omnibull/social-auto-upload).
  OMNIBULL_FACTORY_TMP_DIR     Remote temp extraction directory.
  OMNIDRIVE_BASE_URL           OmniDrive cloud URL passed into setup_factory_master.sh.
  OMNIBULL_LOCAL_OPENCLAW_HOME Local OpenClaw home to mirror (default: ~/.openclaw).

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


ssh_base() {
  ssh -p "${REMOTE_SSH_PORT}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "$@"
}


cleanup_local_bundle() {
  if [[ -n "${LOCAL_OPENCLAW_BUNDLE_DIR}" && -d "${LOCAL_OPENCLAW_BUNDLE_DIR}" ]]; then
    rm -rf "${LOCAL_OPENCLAW_BUNDLE_DIR}"
  fi
}


trap cleanup_local_bundle EXIT


prepare_remote_tree() {
  ssh_base -t "${REMOTE_USER}@${REMOTE_HOST}" "
    set -e
    mkdir -p '${REMOTE_TMP_DIR}'
    sudo install -d -o '${REMOTE_USER}' -g '${REMOTE_USER}' -m 0755 \
      '$(dirname "${REMOTE_APP_ROOT}")' \
      '${REMOTE_APP_ROOT}'
  "
}


prepare_local_openclaw_bundle() {
  LOCAL_OPENCLAW_BUNDLE_DIR="$(mktemp -d)"
  python3 "${ROOT_DIR}/scripts/openclaw_factory_bundle.py" export \
    --source-home "${LOCAL_OPENCLAW_HOME}" \
    --output-dir "${LOCAL_OPENCLAW_BUNDLE_DIR}" \
    --app-root "${REMOTE_APP_ROOT}" \
    --omnidrive-base-url "${OMNIDRIVE_BASE_URL}" >/dev/null
}


push_archive() {
  COPYFILE_DISABLE=1 COPY_EXTENDED_ATTRIBUTES_DISABLE=1 tar \
    --disable-copyfile \
    --no-xattrs \
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
    | ssh_base "${REMOTE_USER}@${REMOTE_HOST}" \
      "rm -rf '${REMOTE_TMP_DIR}/src' && mkdir -p '${REMOTE_TMP_DIR}/src' && tar -xzf - -C '${REMOTE_TMP_DIR}/src'"
}


install_remote_tree() {
  ssh_base -t "${REMOTE_USER}@${REMOTE_HOST}" "
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


push_openclaw_bundle() {
  tar -C "${LOCAL_OPENCLAW_BUNDLE_DIR}" -czf - . \
    | ssh_base "${REMOTE_USER}@${REMOTE_HOST}" \
      "rm -rf '${REMOTE_TMP_DIR}/openclaw-bundle' && mkdir -p '${REMOTE_TMP_DIR}/openclaw-bundle' && tar -xzf - -C '${REMOTE_TMP_DIR}/openclaw-bundle'"
}


run_remote_setup() {
  local setup_cmd
  setup_cmd="cd '${REMOTE_APP_ROOT}' && sudo -E OMNIDRIVE_BASE_URL='${OMNIDRIVE_BASE_URL}' OMNIBULL_OPENCLAW_BUNDLE_DIR='${REMOTE_TMP_DIR}/openclaw-bundle' bash '${REMOTE_APP_ROOT}/scripts/setup_factory_master.sh'"
  ssh_base -t "${REMOTE_USER}@${REMOTE_HOST}" "${setup_cmd}"
}


main() {
  require_cmd ssh
  require_cmd tar
  require_cmd python3

  if [[ -z "${REMOTE_HOST}" ]]; then
    usage
    exit 1
  fi

  prepare_remote_tree
  prepare_local_openclaw_bundle
  push_archive
  install_remote_tree
  push_openclaw_bundle
  run_remote_setup
}


main "$@"
