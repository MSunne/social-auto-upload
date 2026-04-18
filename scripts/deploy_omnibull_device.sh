#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

REMOTE_HOST="${OMNIBULL_FACTORY_HOST:-${1:-}}"
REMOTE_USER="${OMNIBULL_FACTORY_USER:-${2:-sun}}"
REMOTE_SSH_PORT="${OMNIBULL_FACTORY_SSH_PORT:-22}"
REMOTE_TARGET="${REMOTE_USER}@${REMOTE_HOST}"
OMNIDRIVE_BASE_URL="${OMNIDRIVE_BASE_URL:-https://aitoplus.com}"

SSH_OPTS=(
  -p "${REMOTE_SSH_PORT}"
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
  -o ConnectTimeout=5
)


log() {
  printf '[%s] [device-deploy] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}


die() {
  log "ERROR: $*"
  exit 1
}


usage() {
  cat <<EOF
Usage:
  $(basename "$0") <host> [user]

Examples:
  OMNIDRIVE_BASE_URL=https://aitoplus.com $(basename "$0") 192.168.1.20 sun
  $(basename "$0") 192.168.1.20

Environment overrides:
  OMNIBULL_FACTORY_HOST      Remote host or IP.
  OMNIBULL_FACTORY_USER      Remote SSH user (default: sun).
  OMNIBULL_FACTORY_SSH_PORT  Remote SSH port (default: 22).
  OMNIDRIVE_BASE_URL         OmniDrive cloud URL passed into setup_factory_master.sh (default: https://aitoplus.com).

Notes:
  - SSH and sudo will prompt interactively when needed.
  - The script deploys, reboots the device, then reconnects and verifies auto-start.
EOF
}


require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    die "missing required command: ${cmd}"
  fi
}


remote_ssh() {
  ssh "${SSH_OPTS[@]}" "${REMOTE_TARGET}" "$@"
}


remote_ssh_tty() {
  ssh -tt "${SSH_OPTS[@]}" "${REMOTE_TARGET}" "$@"
}


print_ssh_install_hint() {
  cat <<'EOF'
设备未开放 SSH。请先在设备本机执行：
  sudo apt-get update
  sudo apt-get install -y openssh-server
  sudo systemctl enable --now ssh
EOF
}


check_local_prerequisites() {
  require_cmd ssh
  require_cmd ping
  require_cmd nc
  require_cmd curl
  require_cmd tar
}


check_host_reachable() {
  log "checking network reachability for ${REMOTE_HOST}"
  ping -c 1 -W 2 "${REMOTE_HOST}" >/dev/null 2>&1 \
    || die "host ${REMOTE_HOST} is unreachable"
}


check_ssh_port() {
  log "checking SSH port ${REMOTE_SSH_PORT} on ${REMOTE_HOST}"
  if ! nc -zvw2 "${REMOTE_HOST}" "${REMOTE_SSH_PORT}" >/dev/null 2>&1; then
    print_ssh_install_hint
    die "SSH port ${REMOTE_SSH_PORT} is not accepting connections on ${REMOTE_HOST}"
  fi
}


check_ssh_login() {
  log "checking SSH login for ${REMOTE_TARGET}"
  remote_ssh_tty "printf 'SSH login ok\n'"
}


check_remote_os() {
  local remote_os
  log "checking remote operating system"
  remote_os="$(remote_ssh ". /etc/os-release && printf '%s\n' \"\${ID:-unknown}\"")"
  case "${remote_os}" in
    deepin|debian|ubuntu)
      ;;
    *)
      die "unsupported remote OS id '${remote_os}' on ${REMOTE_HOST}"
      ;;
  esac
}


check_remote_sudo() {
  log "checking sudo access for ${REMOTE_TARGET}"
  remote_ssh_tty "sudo -v && sudo -n true"
}


run_remote_prechecks() {
  check_host_reachable
  check_ssh_port
  check_ssh_login
  check_remote_os
  check_remote_sudo
}


deploy_remote_tree() {
  log "syncing workspace and running setup on ${REMOTE_HOST}"
  (
    cd "${ROOT_DIR}"
    OMNIDRIVE_BASE_URL="${OMNIDRIVE_BASE_URL}" \
    OMNIBULL_FACTORY_HOST="${REMOTE_HOST}" \
    OMNIBULL_FACTORY_USER="${REMOTE_USER}" \
    OMNIBULL_FACTORY_SSH_PORT="${REMOTE_SSH_PORT}" \
    "${SCRIPT_DIR}/push_factory_master.sh" "${REMOTE_HOST}" "${REMOTE_USER}"
  )
}


request_remote_reboot() {
  log "requesting remote reboot"
  remote_ssh_tty "sudo systemctl reboot" || true
}


wait_for_ssh_down() {
  local timeout_seconds="${1:-90}"
  local deadline=$((SECONDS + timeout_seconds))

  log "waiting for ${REMOTE_HOST} SSH to go down"
  while (( SECONDS < deadline )); do
    if ! nc -z "${REMOTE_HOST}" "${REMOTE_SSH_PORT}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  die "timed out waiting for ${REMOTE_HOST} SSH to stop during reboot"
}


wait_for_ssh_up() {
  local timeout_seconds="${1:-240}"
  local deadline=$((SECONDS + timeout_seconds))

  log "waiting for ${REMOTE_HOST} SSH to come back"
  while (( SECONDS < deadline )); do
    if nc -z "${REMOTE_HOST}" "${REMOTE_SSH_PORT}" >/dev/null 2>&1; then
      if remote_ssh "printf 'SSH ready\n'" >/dev/null 2>&1; then
        return 0
      fi
    fi
    sleep 3
  done

  die "timed out waiting for ${REMOTE_HOST} SSH to come back after reboot"
}


verify_remote_state() {
  local remote_script

  log "verifying remote service, ports, permissions, and logs"
  remote_script="$(cat <<'REMOTE'
set -euo pipefail

wait_for_http_ready() {
  local url="$1"
  local timeout_seconds="${2:-60}"
  local deadline=$((SECONDS + timeout_seconds))

  while (( SECONDS < deadline )); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "timed out waiting for ${url}" >&2
  return 1
}

enabled_state="$(systemctl is-enabled sau-stack.service)"
active_state="$(systemctl is-active sau-stack.service)"
[[ "${enabled_state}" == "enabled" ]]
[[ "${active_state}" == "active" ]]

wait_for_http_ready "http://127.0.0.1:5173/"
wait_for_http_ready "http://127.0.0.1:5409/omnidriveAgentStatus"

ss -ltnp | grep -q ':22'
ss -ltnp | grep -q ':5409'
ss -ltnp | grep -q ':5173'

command -v google-chrome >/dev/null
desktop_dir="$(xdg-user-dir DESKTOP 2>/dev/null || printf '%s\n' '/home/sun/Desktop')"
chrome_desktop_file="${desktop_dir}/Google Chrome.desktop"
[[ -x "${chrome_desktop_file}" ]]

for path in \
  /persistent/omnibull/logs \
  /persistent/omnibull/runtime \
  /persistent/omnibull/workspace
do
  owner="$(stat -c '%U:%G' "${path}")"
  [[ "${owner}" == "sun:sun" ]]
done

if sudo journalctl -u sau-stack.service -b --no-pager | grep -E 'Permission denied|权限不够' >/dev/null; then
  echo "permission-related journal entries detected for sau-stack.service" >&2
  exit 1
fi
REMOTE
)"

  remote_ssh_tty "bash -lc $(printf '%q' "${remote_script}")"
}


main() {
  check_local_prerequisites

  if [[ -z "${REMOTE_HOST}" ]]; then
    usage
    exit 1
  fi

  log "starting deployment to ${REMOTE_TARGET}"
  run_remote_prechecks
  deploy_remote_tree
  request_remote_reboot
  wait_for_ssh_down
  wait_for_ssh_up
  verify_remote_state
  log "设备 ${REMOTE_HOST} 已完成，可关机并发送下一台 IP"
}


main "$@"
