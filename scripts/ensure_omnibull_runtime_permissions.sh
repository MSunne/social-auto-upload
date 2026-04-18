#!/usr/bin/env bash

set -euo pipefail

APP_USER="${OMNIBULL_APP_USER:-sun}"
APP_GROUP="${OMNIBULL_APP_GROUP:-${APP_USER}}"
PERSISTENT_ROOT="${OMNIBULL_PERSISTENT_ROOT:-/persistent/omnibull}"
RUNTIME_DIR="${OMNIBULL_RUNTIME_DIR:-${PERSISTENT_ROOT}/runtime}"
LOG_DIR="${OMNIBULL_LOG_DIR:-${PERSISTENT_ROOT}/logs}"
WORKSPACE_DIR="${OMNIBULL_WORKSPACE_DIR:-${PERSISTENT_ROOT}/workspace}"


require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    printf 'ensure_omnibull_runtime_permissions.sh must run as root\n' >&2
    exit 1
  fi
}


ensure_dir_tree() {
  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 \
    "${PERSISTENT_ROOT}" \
    "${LOG_DIR}" \
    "${RUNTIME_DIR}" \
    "${RUNTIME_DIR}/cookiesFile" \
    "${RUNTIME_DIR}/taskArtifacts" \
    "${WORKSPACE_DIR}" \
    "${WORKSPACE_DIR}/videoFile" \
    "${WORKSPACE_DIR}/omnidriveSync"

  chown -R "${APP_USER}:${APP_GROUP}" \
    "${LOG_DIR}" \
    "${RUNTIME_DIR}" \
    "${WORKSPACE_DIR}"
  find "${LOG_DIR}" "${RUNTIME_DIR}" "${WORKSPACE_DIR}" -type d -exec chmod 0755 {} +
}


main() {
  require_root
  ensure_dir_tree
}


main "$@"
