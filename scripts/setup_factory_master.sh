#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

APP_USER="${OMNIBULL_APP_USER:-sun}"
APP_GROUP="${OMNIBULL_APP_GROUP:-${APP_USER}}"
APP_HOME="${OMNIBULL_APP_HOME:-$(getent passwd "${APP_USER}" | cut -d: -f6)}"
APP_ROOT="${OMNIBULL_APP_ROOT:-/opt/omnibull/social-auto-upload}"
VENV_DIR="${OMNIBULL_VENV_DIR:-/opt/omnibull/venv}"
PLAYWRIGHT_DIR="${OMNIBULL_PLAYWRIGHT_DIR:-/opt/playwright}"
NODE_DIR="${OMNIBULL_NODE_DIR:-/opt/node-v22}"
PERSISTENT_ROOT="${OMNIBULL_PERSISTENT_ROOT:-/persistent/omnibull}"
RUNTIME_DIR="${OMNIBULL_RUNTIME_DIR:-${PERSISTENT_ROOT}/runtime}"
LOG_DIR="${OMNIBULL_LOG_DIR:-${PERSISTENT_ROOT}/logs}"
WORKSPACE_DIR="${OMNIBULL_WORKSPACE_DIR:-${PERSISTENT_ROOT}/workspace}"
VIDEO_DIR="${OMNIBULL_VIDEO_DIR:-${WORKSPACE_DIR}/videoFile}"
COOKIE_DIR="${OMNIBULL_COOKIE_DIR:-${RUNTIME_DIR}/cookiesFile}"
ARTIFACT_DIR="${OMNIBULL_ARTIFACT_DIR:-${RUNTIME_DIR}/taskArtifacts}"
SYNC_DIR="${OMNIBULL_SYNC_DIR:-${WORKSPACE_DIR}/omnidriveSync}"
DB_PATH="${OMNIBULL_DB_PATH:-${RUNTIME_DIR}/database.db}"
CHROME_DESKTOP_FILE="${OMNIBULL_CHROME_DESKTOP_FILE:-}"
LIGHTDM_AUTLOGIN_FILE="${OMNIBULL_LIGHTDM_AUTLOGIN_FILE:-/etc/lightdm/lightdm.conf.d/90-omnibull-autologin.conf}"
ENV_FILE="${OMNIBULL_ENV_FILE:-/etc/omnibull/omnibull.env}"
CONF_FILE="${OMNIBULL_CONF_FILE:-${APP_ROOT}/conf.py}"

NODE_DIST_BASE_URL="${OMNIBULL_NODE_DIST_BASE_URL:-https://nodejs.org/dist/latest-v22.x}"
CHROME_DEB_URL="${OMNIBULL_CHROME_DEB_URL:-https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb}"
PIP_INDEX_URL="${OMNIBULL_PIP_INDEX_URL:-https://mirrors.aliyun.com/pypi/simple/}"
NPM_REGISTRY="${OMNIBULL_NPM_REGISTRY:-https://registry.npmmirror.com}"
OMNIDRIVE_BASE_URL="${OMNIDRIVE_BASE_URL:-}"
OMNIBULL_CORS_ALLOWED_ORIGINS="${OMNIBULL_CORS_ALLOWED_ORIGINS:-}"
OMNIBULL_SETUP_RESTART_SERVICE="${OMNIBULL_SETUP_RESTART_SERVICE:-1}"
OMNIBULL_SETUP_VERIFY="${OMNIBULL_SETUP_VERIFY:-1}"
APP_USER_SWITCH_TOOL=""
APT_CMD=(apt-get)
APT_TEMP_DIR=""
OS_ID=""
OS_CODENAME=""

APT_PACKAGES=(
  python3
  python3-pip
  python3-venv
  curl
  xz-utils
  ca-certificates
  build-essential
)


log() {
  printf '[%s] [factory-setup] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}


cleanup_apt_temp_dir() {
  if [[ -n "${APT_TEMP_DIR}" && -d "${APT_TEMP_DIR}" ]]; then
    rm -rf "${APT_TEMP_DIR}"
  fi
}


trap cleanup_apt_temp_dir EXIT


require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    log "missing required command: ${cmd}"
    exit 1
  fi
}


resolve_user_switch_tool() {
  if command -v runuser >/dev/null 2>&1; then
    APP_USER_SWITCH_TOOL="runuser"
    return 0
  fi
  if command -v su >/dev/null 2>&1; then
    APP_USER_SWITCH_TOOL="su"
    return 0
  fi
  log "missing required command: runuser or su"
  exit 1
}


run_as_app_user() {
  local -a env_pairs=()
  local -a cmd=()
  local -a wrapped=(env)
  local quoted=""

  while [[ "$#" -gt 0 ]]; do
    if [[ "$1" == "--" ]]; then
      shift
      break
    fi
    env_pairs+=("$1")
    shift
  done
  cmd=("$@")
  if [[ "${#cmd[@]}" -eq 0 ]]; then
    log "run_as_app_user requires a command"
    exit 1
  fi

  if [[ "${APP_USER_SWITCH_TOOL}" == "runuser" ]]; then
    runuser -u "${APP_USER}" -- env "${env_pairs[@]}" "${cmd[@]}"
    return 0
  fi

  if [[ "${#env_pairs[@]}" -gt 0 ]]; then
    wrapped+=("${env_pairs[@]}")
  fi
  wrapped+=("${cmd[@]}")
  printf -v quoted '%q ' "${wrapped[@]}"
  su -s /bin/bash "${APP_USER}" -c "${quoted% }"
}


resolve_desktop_dir() {
  local desktop_dir=""

  if command -v xdg-user-dir >/dev/null 2>&1; then
    desktop_dir="$(run_as_app_user HOME="${APP_HOME}" -- xdg-user-dir DESKTOP 2>/dev/null || true)"
  fi

  if [[ -z "${desktop_dir}" || "${desktop_dir}" == "${APP_HOME}" ]]; then
    desktop_dir="${APP_HOME}/Desktop"
  fi

  printf '%s\n' "${desktop_dir}"
}


resolve_chrome_desktop_file() {
  if [[ -n "${CHROME_DESKTOP_FILE}" ]]; then
    printf '%s\n' "${CHROME_DESKTOP_FILE}"
    return 0
  fi

  printf '%s/Google Chrome.desktop\n' "$(resolve_desktop_dir)"
}


ensure_prerequisites() {
  require_cmd apt-get
  require_cmd python3
  require_cmd tar
  require_cmd install
  require_cmd systemctl
  require_cmd gsettings
  resolve_user_switch_tool
}


load_os_release() {
  OS_ID=""
  OS_CODENAME=""
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    OS_ID="${ID:-}"
    OS_CODENAME="${VERSION_CODENAME:-${UBUNTU_CODENAME:-${VERSION_ID:-}}}"
  fi
}


build_aliyun_apt_sources() {
  case "${OS_ID}" in
    deepin)
      [[ -n "${OS_CODENAME}" ]] || return 1
      cat <<EOF
deb https://mirrors.aliyun.com/deepin/ ${OS_CODENAME} main commercial community
EOF
      ;;
    ubuntu)
      [[ -n "${OS_CODENAME}" ]] || return 1
      cat <<EOF
deb https://mirrors.aliyun.com/ubuntu/ ${OS_CODENAME} main restricted universe multiverse
deb https://mirrors.aliyun.com/ubuntu/ ${OS_CODENAME}-updates main restricted universe multiverse
deb https://mirrors.aliyun.com/ubuntu/ ${OS_CODENAME}-backports main restricted universe multiverse
deb https://mirrors.aliyun.com/ubuntu/ ${OS_CODENAME}-security main restricted universe multiverse
EOF
      ;;
    debian)
      [[ -n "${OS_CODENAME}" ]] || return 1
      cat <<EOF
deb https://mirrors.aliyun.com/debian/ ${OS_CODENAME} main contrib non-free non-free-firmware
deb https://mirrors.aliyun.com/debian/ ${OS_CODENAME}-updates main contrib non-free non-free-firmware
deb https://mirrors.aliyun.com/debian/ ${OS_CODENAME}-backports main contrib non-free non-free-firmware
deb https://mirrors.aliyun.com/debian-security/ ${OS_CODENAME}-security main contrib non-free non-free-firmware
EOF
      ;;
    *)
      return 1
      ;;
  esac
}


configure_apt_command() {
  local sources_file

  load_os_release
  sources_file="$(build_aliyun_apt_sources)" || {
    log "using system apt sources for unsupported mirror preset: ${OS_ID:-unknown}"
    return 0
  }

  cleanup_apt_temp_dir
  APT_TEMP_DIR="$(mktemp -d)"
  printf '%s\n' "${sources_file}" > "${APT_TEMP_DIR}/aliyun.sources.list"
  APT_CMD=(
    apt-get
    -o "Dir::Etc::sourcelist=${APT_TEMP_DIR}/aliyun.sources.list"
    -o Dir::Etc::sourceparts=-
    -o Acquire::Retries=3
  )
  log "using Aliyun apt mirror for ${OS_ID} ${OS_CODENAME}"
}


read_existing_conf_value() {
  local key="$1"
  local conf_path="${2:-${CONF_FILE}}"
  [[ -f "${conf_path}" ]] || return 0
  python3 - "${conf_path}" "${key}" <<'PY'
import runpy
import sys

path, key = sys.argv[1], sys.argv[2]
try:
    values = runpy.run_path(path)
except Exception:
    sys.exit(0)
value = values.get(key)
if value is None:
    sys.exit(0)
print(value)
PY
}


ensure_config_values() {
  if [[ -z "${OMNIDRIVE_BASE_URL}" ]]; then
    OMNIDRIVE_BASE_URL="$(read_existing_conf_value "OMNIDRIVE_BASE_URL")"
  fi
  if [[ -z "${OMNIBULL_CORS_ALLOWED_ORIGINS}" ]]; then
    OMNIBULL_CORS_ALLOWED_ORIGINS="$(read_existing_conf_value "OMNIBULL_CORS_ALLOWED_ORIGINS")"
  fi
  OMNIBULL_CORS_ALLOWED_ORIGINS="${OMNIBULL_CORS_ALLOWED_ORIGINS:-*}"

  if [[ -z "${APP_HOME}" || ! -d "${APP_HOME}" ]]; then
    log "failed to resolve home directory for ${APP_USER}"
    exit 1
  fi
  if [[ -z "${OMNIDRIVE_BASE_URL}" ]]; then
    log "OMNIDRIVE_BASE_URL is required"
    exit 1
  fi
}


install_system_packages() {
  log "installing required Debian/Deepin packages"
  export DEBIAN_FRONTEND=noninteractive
  "${APT_CMD[@]}" update
  "${APT_CMD[@]}" install -y "${APT_PACKAGES[@]}"
}


detect_node_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf 'x64\n'
      ;;
    aarch64|arm64)
      printf 'arm64\n'
      ;;
    *)
      log "unsupported CPU architecture for Node.js: $(uname -m)"
      exit 1
      ;;
  esac
}


node_bin_is_compatible() {
  local node_bin="$1"
  [[ -x "${node_bin}" ]] || return 1
  "${node_bin}" -e 'process.exit(Number(process.versions.node.split(".")[0]) >= 22 ? 0 : 1)'
}


install_node_runtime() {
  local node_arch
  local node_index
  local node_prefix
  local node_tarball
  local node_url
  local tmp_dir

  if node_bin_is_compatible "${NODE_DIR}/bin/node"; then
    log "Node.js runtime already satisfies v22 requirement"
    return 0
  fi

  node_arch="$(detect_node_arch)"
  node_index="$(curl -fsSL "${NODE_DIST_BASE_URL}/SHASUMS256.txt")"
  node_prefix="$(
    printf '%s\n' "${node_index}" \
      | awk -v arch="${node_arch}" '$2 ~ ("linux-" arch "\\.tar\\.xz$") { sub("-linux-" arch "\\.tar\\.xz$", "", $2); print $2; exit }'
  )"
  if [[ -z "${node_prefix}" ]]; then
    log "failed to resolve Node.js v22 package for ${node_arch}"
    exit 1
  fi

  node_tarball="${node_prefix}-linux-${node_arch}.tar.xz"
  node_url="${NODE_DIST_BASE_URL}/${node_tarball}"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "'"${tmp_dir}"'"' RETURN

  log "installing Node.js ${node_prefix} into ${NODE_DIR}"
  curl -fsSL "${node_url}" | tar -xJf - -C "${tmp_dir}"
  rm -rf "${NODE_DIR}"
  mv "${tmp_dir}/${node_prefix}-linux-${node_arch}" "${NODE_DIR}"
  rm -rf "${tmp_dir}"
  trap - RETURN
}


install_google_chrome() {
  local deb_path
  if command -v google-chrome >/dev/null 2>&1; then
    log "Google Chrome is already installed"
    return 0
  fi

  deb_path="$(mktemp --suffix=.deb)"
  trap 'rm -f "'"${deb_path}"'"' RETURN
  log "installing Google Chrome stable"
  curl -fsSL "${CHROME_DEB_URL}" -o "${deb_path}"
  export DEBIAN_FRONTEND=noninteractive
  "${APT_CMD[@]}" install -y "${deb_path}"
  rm -f "${deb_path}"
  trap - RETURN
}


ensure_directories() {
  log "creating persistent runtime directories"
  install -d -o "${APP_USER}" -g "${APP_GROUP}" -m 0755 \
    "${APP_ROOT}" \
    "${VENV_DIR}" \
    "${PLAYWRIGHT_DIR}" \
    "${PERSISTENT_ROOT}" \
    "${RUNTIME_DIR}" \
    "${LOG_DIR}" \
    "${WORKSPACE_DIR}" \
    "${VIDEO_DIR}" \
    "${COOKIE_DIR}" \
    "${ARTIFACT_DIR}" \
    "${SYNC_DIR}" \
    "$(dirname "${DB_PATH}")"

  install -d -o root -g "${APP_GROUP}" -m 0775 /etc/omnibull
}


copy_tree_if_needed() {
  local source_path="$1"
  local target_path="$2"
  if [[ ! -e "${source_path}" || -L "${source_path}" ]]; then
    return 0
  fi
  if [[ -d "${source_path}" ]]; then
    mkdir -p "${target_path}"
    cp -a "${source_path}/." "${target_path}/"
    rm -rf "${source_path}"
    return 0
  fi
  mkdir -p "$(dirname "${target_path}")"
  cp -a "${source_path}" "${target_path}"
  rm -f "${source_path}"
}


ensure_link() {
  local link_path="$1"
  local target_path="$2"
  mkdir -p "$(dirname "${link_path}")"
  if [[ -L "${link_path}" ]]; then
    ln -sfn "${target_path}" "${link_path}"
    return 0
  fi
  copy_tree_if_needed "${link_path}" "${target_path}"
  ln -sfn "${target_path}" "${link_path}"
}


wire_runtime_paths() {
  log "rewiring mutable runtime paths into ${PERSISTENT_ROOT}"
  ensure_link "${APP_ROOT}/logs" "${LOG_DIR}"
  ensure_link "${APP_ROOT}/runtime" "${RUNTIME_DIR}"
  ensure_link "${APP_ROOT}/videoFile" "${VIDEO_DIR}"
  ensure_link "${APP_ROOT}/cookiesFile" "${COOKIE_DIR}"
  ensure_link "${APP_ROOT}/taskArtifacts" "${ARTIFACT_DIR}"
  ensure_link "${APP_ROOT}/omnidriveSync" "${SYNC_DIR}"
  ensure_link "${APP_ROOT}/db/database.db" "${DB_PATH}"
}


normalize_persistent_permissions() {
  log "normalizing ownership for mutable OmniBull state"
  if [[ -d "${PERSISTENT_ROOT}" ]]; then
    chown -R "${APP_USER}:${APP_GROUP}" "${PERSISTENT_ROOT}"
    find "${PERSISTENT_ROOT}" -type d -exec chmod 0755 {} +
  fi
}


write_conf_file() {
  log "writing production conf.py"
  cat > "${CONF_FILE}" <<EOF
from pathlib import Path

BASE_DIR = Path(__file__).parent.resolve()
XHS_SERVER = "http://127.0.0.1:11901"
LOCAL_CHROME_PATH = "/usr/bin/google-chrome"
LOCAL_CHROME_HEADLESS = False
SAU_LOG_LEVEL = "INFO"

CLOUD_AGENT_ENABLED = False
CLOUD_DEMO_URL = ""
CLOUD_DEVICE_NAME = ""
CLOUD_DEVICE_CODE = ""
CLOUD_AGENT_KEY = ""
CLOUD_AGENT_POLL_INTERVAL = 5
CLOUD_AGENT_HEARTBEAT_INTERVAL = 30

OMNIDRIVE_AGENT_ENABLED = True
OMNIDRIVE_BASE_URL = "${OMNIDRIVE_BASE_URL}"
OMNIDRIVE_AGENT_KEY = ""
OMNIDRIVE_AGENT_POLL_INTERVAL = 5
OMNIDRIVE_AGENT_HEARTBEAT_INTERVAL = 30
OMNIDRIVE_ACCOUNT_SYNC_INTERVAL = 60
OMNIDRIVE_ACCOUNT_VALIDATION_INTERVAL = 21600
OMNIDRIVE_MATERIAL_SYNC_INTERVAL = 300
OMNIDRIVE_SKILL_SYNC_INTERVAL = 120
OMNIDRIVE_PUBLISH_SYNC_INTERVAL = 5
OMNIDRIVE_MATERIAL_SYNC_MAX_FILES = 1000
OMNIBULL_DEVICE_IDENTITY_FILE = "/etc/omnibull/device.json"

OMNIBULL_PUBLISH_WORKERS = 1
OMNIBULL_PUBLISH_DISPATCH_INTERVAL_SECONDS = 5
OMNIBULL_TASK_RETENTION_DAYS = 7
OMNIBULL_API_KEY = ""
OMNIBULL_CORS_ALLOWED_ORIGINS = "${OMNIBULL_CORS_ALLOWED_ORIGINS}"
OMNIBULL_CORS_ALLOWED_METHODS = "GET,POST,PUT,PATCH,DELETE,OPTIONS"
OMNIBULL_CORS_ALLOWED_HEADERS = "Authorization,Content-Type,X-Requested-With,X-Omnibull-Key"
OMNIBULL_CORS_EXPOSE_HEADERS = "Content-Disposition,X-Accel-Buffering"
OMNIBULL_CORS_ALLOW_CREDENTIALS = False
OMNIBULL_CORS_MAX_AGE = 86400
OMNIBULL_MATERIAL_ROOTS = {
    "localWorkspace": "${WORKSPACE_DIR}",
}
EOF
  chown "${APP_USER}:${APP_GROUP}" "${CONF_FILE}"
  chmod 0644 "${CONF_FILE}"
}


write_env_file() {
  local user_id
  user_id="$(id -u "${APP_USER}")"
  log "writing ${ENV_FILE}"
  cat > "${ENV_FILE}" <<EOF
HOME=${APP_HOME}
PATH=${NODE_DIR}/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
PYTHONUNBUFFERED=1
DISPLAY=:0
XAUTHORITY=${APP_HOME}/.Xauthority
XDG_RUNTIME_DIR=/run/user/${user_id}
DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/${user_id}/bus
PLAYWRIGHT_BROWSERS_PATH=${PLAYWRIGHT_DIR}
SAU_PYTHON_BIN=${VENV_DIR}/bin/python
SAU_NPM_BIN=${NODE_DIR}/bin/npm
SAU_BACKEND_PORT=5409
SAU_FRONTEND_HOST=0.0.0.0
SAU_FRONTEND_PORT=5173
SAU_FRONTEND_MODE=preview
SAU_FRONTEND_BUILD_ON_START=never
SAU_FRONTEND_INSTALL_ON_START=0
SAU_RUNTIME_DIR=${RUNTIME_DIR}
SAU_LOG_DIR=${LOG_DIR}
SAU_OPEN_DESKTOP_BROWSER=0
EOF
  chmod 0644 "${ENV_FILE}"
}


create_python_venv() {
  if [[ ! -x "${VENV_DIR}/bin/python" ]]; then
    log "creating Python virtual environment"
    python3 -m venv "${VENV_DIR}"
  fi

  log "installing Python dependencies"
  PIP_CONFIG_FILE=/dev/null "${VENV_DIR}/bin/pip" install --index-url "${PIP_INDEX_URL}" --upgrade pip
  PIP_CONFIG_FILE=/dev/null "${VENV_DIR}/bin/pip" install --index-url "${PIP_INDEX_URL}" -r "${APP_ROOT}/requirements.txt"
}


install_frontend_dependencies() {
  log "installing sau_frontend dependencies"
  run_as_app_user HOME="${APP_HOME}" PATH="${NODE_DIR}/bin:/usr/bin:/bin" \
    -- "${NODE_DIR}/bin/npm" config set registry "${NPM_REGISTRY}"
  if [[ -f "${APP_ROOT}/sau_frontend/package-lock.json" ]]; then
    run_as_app_user HOME="${APP_HOME}" PATH="${NODE_DIR}/bin:/usr/bin:/bin" \
      npm_config_registry="${NPM_REGISTRY}" \
      -- "${NODE_DIR}/bin/npm" --prefix "${APP_ROOT}/sau_frontend" ci
  else
    run_as_app_user HOME="${APP_HOME}" PATH="${NODE_DIR}/bin:/usr/bin:/bin" \
      npm_config_registry="${NPM_REGISTRY}" \
      -- "${NODE_DIR}/bin/npm" --prefix "${APP_ROOT}/sau_frontend" install
  fi

  log "building sau_frontend preview assets"
  run_as_app_user HOME="${APP_HOME}" PATH="${NODE_DIR}/bin:/usr/bin:/bin" \
    npm_config_registry="${NPM_REGISTRY}" \
    -- "${NODE_DIR}/bin/npm" --prefix "${APP_ROOT}/sau_frontend" run build
}


install_playwright_browser() {
  log "installing Playwright Chromium into ${PLAYWRIGHT_DIR}"
  PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_DIR}" "${VENV_DIR}/bin/python" -m playwright install chromium
}


initialize_database() {
  if [[ ! -L "${APP_ROOT}/db/database.db" ]]; then
    ensure_link "${APP_ROOT}/db/database.db" "${DB_PATH}"
  fi

  log "initializing SQLite schema"
  (
    cd "${APP_ROOT}/db"
    OMNIBULL_DB_PATH="${DB_PATH}" "${VENV_DIR}/bin/python" createTable.py
  )
  chown -h "${APP_USER}:${APP_GROUP}" "${APP_ROOT}/db/database.db"
  chown "${APP_USER}:${APP_GROUP}" "${DB_PATH}" 2>/dev/null || true
}


copy_chrome_shortcut() {
  local source_desktop
  local target_desktop_file
  source_desktop=""
  if [[ -f /usr/share/applications/google-chrome.desktop ]]; then
    source_desktop="/usr/share/applications/google-chrome.desktop"
  elif [[ -f /usr/share/applications/google-chrome-stable.desktop ]]; then
    source_desktop="/usr/share/applications/google-chrome-stable.desktop"
  fi
  [[ -n "${source_desktop}" ]] || return 0

  target_desktop_file="$(resolve_chrome_desktop_file)"
  mkdir -p "$(dirname "${target_desktop_file}")"
  cp -f "${source_desktop}" "${target_desktop_file}"
  chown "${APP_USER}:${APP_GROUP}" "${target_desktop_file}"
  chmod 0755 "${target_desktop_file}"
}


configure_lightdm_autologin() {
  log "configuring LightDM autologin for ${APP_USER}"
  mkdir -p "$(dirname "${LIGHTDM_AUTLOGIN_FILE}")"
  cat > "${LIGHTDM_AUTLOGIN_FILE}" <<EOF
[Seat:*]
autologin-user=${APP_USER}
autologin-user-timeout=0
EOF
}


run_gsettings_as_app() {
  local schema="$1"
  local key="$2"
  local value="$3"
  local user_id
  user_id="$(id -u "${APP_USER}")"

  run_as_app_user \
    HOME="${APP_HOME}" \
    DISPLAY=:0 \
    XAUTHORITY="${APP_HOME}/.Xauthority" \
    XDG_RUNTIME_DIR="/run/user/${user_id}" \
    DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/${user_id}/bus" \
    -- gsettings set "${schema}" "${key}" "${value}" >/dev/null 2>&1 \
    || run_as_app_user HOME="${APP_HOME}" -- dbus-run-session \
      gsettings set "${schema}" "${key}" "${value}" >/dev/null 2>&1 \
    || true
}


configure_desktop_policy() {
  log "configuring desktop autologin, lockscreen, and power policy"
  configure_lightdm_autologin
  loginctl enable-linger "${APP_USER}" >/dev/null 2>&1 || true
  systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target >/dev/null 2>&1 || true

  run_gsettings_as_app com.deepin.dde.power line-power-sleep-delay 0
  run_gsettings_as_app com.deepin.dde.power line-power-lock-delay 0
  run_gsettings_as_app com.deepin.dde.power line-power-screen-black-delay 0
  run_gsettings_as_app com.deepin.dde.power line-power-screensaver-delay 0
  run_gsettings_as_app com.deepin.dde.power battery-sleep-delay 0
  run_gsettings_as_app com.deepin.dde.power battery-lock-delay 0
  run_gsettings_as_app com.deepin.dde.power battery-screen-black-delay 0
  run_gsettings_as_app com.deepin.dde.power battery-screensaver-delay 0
  run_gsettings_as_app com.deepin.dde.power screen-black-lock false
  run_gsettings_as_app com.deepin.dde.power sleep-lock false
  run_gsettings_as_app com.deepin.dde.power lid-closed-sleep false
  run_gsettings_as_app com.deepin.dde.power battery-lid-closed-sleep false
  run_gsettings_as_app com.deepin.wrap.gnome.desktop.session idle-delay "uint32 0"
  run_gsettings_as_app com.deepin.wrap.gnome.desktop.screensaver lock-enabled false
  run_gsettings_as_app com.deepin.wrap.gnome.desktop.screensaver idle-activation-enabled false
  run_gsettings_as_app com.deepin.wrap.gnome.desktop.screensaver lock-delay "uint32 0"
}


install_service() {
  log "installing systemd service"
  install -m 0644 "${ROOT_DIR}/deploy/systemd/sau-stack.service" /etc/systemd/system/sau-stack.service
  systemctl daemon-reload
  systemctl disable --now omnibull.service >/dev/null 2>&1 || true
  systemctl enable sau-stack.service >/dev/null 2>&1 || true
}


restart_service_if_needed() {
  if [[ "${OMNIBULL_SETUP_RESTART_SERVICE}" != "1" ]]; then
    log "skipping service restart because OMNIBULL_SETUP_RESTART_SERVICE=${OMNIBULL_SETUP_RESTART_SERVICE}"
    return 0
  fi

  log "restarting sau-stack.service"
  systemctl restart sau-stack.service
}


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

  log "timed out waiting for ${url}"
  return 1
}


verify_installation() {
  local browser_path
  local chrome_desktop_file
  local verify_path
  local user_id
  [[ "${OMNIBULL_SETUP_VERIFY}" == "1" ]] || return 0

  log "running installation verification"
  verify_path="${NODE_DIR}/bin:${PATH}"
  chrome_desktop_file="$(resolve_chrome_desktop_file)"
  user_id="$(id -u "${APP_USER}")"
  python3 --version >/dev/null
  "${NODE_DIR}/bin/node" --version >/dev/null
  env PATH="${verify_path}" "${NODE_DIR}/bin/npm" --version >/dev/null
  google-chrome --version >/dev/null
  [[ -x "${chrome_desktop_file}" ]]
  systemctl is-active --quiet sau-stack.service
  wait_for_http_ready "http://127.0.0.1:5409/omnidriveAgentStatus"
  wait_for_http_ready "http://127.0.0.1:5173/"

  browser_path="$(run_as_app_user \
    HOME="${APP_HOME}" \
    DISPLAY=:0 \
    XAUTHORITY="${APP_HOME}/.Xauthority" \
    XDG_RUNTIME_DIR="/run/user/${user_id}" \
    DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/${user_id}/bus" \
    PLAYWRIGHT_BROWSERS_PATH="${PLAYWRIGHT_DIR}" \
    SAU_LOG_DIR="${LOG_DIR}" \
    -- "${VENV_DIR}/bin/python" -c \
    'from utils.browser_hook import get_browser_options; print(get_browser_options(headless=False)["executable_path"])' \
    | tail -n 1)"
  if [[ ! -x "${browser_path}" ]]; then
    log "browser verification failed; resolved path is not executable: ${browser_path}"
    exit 1
  fi
}


main() {
  ensure_prerequisites
  configure_apt_command
  ensure_config_values
  install_system_packages
  install_node_runtime
  install_google_chrome
  ensure_directories
  wire_runtime_paths
  normalize_persistent_permissions
  write_conf_file
  write_env_file
  create_python_venv
  install_frontend_dependencies
  install_playwright_browser
  initialize_database
  normalize_persistent_permissions
  copy_chrome_shortcut
  configure_desktop_policy
  install_service
  normalize_persistent_permissions
  restart_service_if_needed
  verify_installation
  log "factory master setup completed"
}


main "$@"
