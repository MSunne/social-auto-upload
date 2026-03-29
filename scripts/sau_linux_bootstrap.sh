#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

APT_PACKAGES=(
  python3
  python3-pip
  python3-venv
  chromium
  curl
  xz-utils
)
DEFAULT_PIP_INDEX_URL="${SAU_PIP_INDEX_URL:-${PIP_INDEX_URL:-https://pypi.tuna.tsinghua.edu.cn/simple}}"
DEFAULT_NODE_DIST_BASE_URL="${SAU_NODE_DIST_BASE_URL:-https://nodejs.org/dist/latest-v22.x}"
OPENCLAW_INSTALL_ON_BOOTSTRAP="${SAU_INSTALL_OPENCLAW:-1}"


log() {
  printf '[%s] [bootstrap] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}


run_as_root() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return 0
  fi
  sudo "$@"
}


ensure_apt() {
  if command -v apt-get >/dev/null 2>&1; then
    return 0
  fi
  log "apt-get not found, bootstrap currently supports Debian/Deepin/Ubuntu only"
  exit 1
}


install_system_packages() {
  log "Installing Linux runtime packages: ${APT_PACKAGES[*]}"
  export DEBIAN_FRONTEND=noninteractive
  run_as_root apt-get update
  run_as_root apt-get install -y "${APT_PACKAGES[@]}"
}


ensure_venv() {
  if [[ -x "${ROOT_DIR}/.venv/bin/python" ]]; then
    return 0
  fi
  log "Creating Python virtual environment"
  python3 -m venv "${ROOT_DIR}/.venv"
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
      log "Unsupported Linux architecture for Node.js bootstrap: $(uname -m)"
      exit 1
      ;;
  esac
}


node_bin_is_compatible() {
  local node_bin="$1"
  [[ -x "${node_bin}" ]] || return 1
  "${node_bin}" -e 'const major = Number(process.versions.node.split(".")[0]); process.exit(major >= 22 ? 0 : 1);'
}


install_node_runtime() {
  local node_arch
  local node_index
  local node_prefix
  local node_tarball
  local node_url
  local install_root="${HOME}/.local"
  local install_link="${install_root}/node-current"

  node_arch="$(detect_node_arch)"
  node_index="$(curl -fsSL "${DEFAULT_NODE_DIST_BASE_URL}/SHASUMS256.txt")"
  node_prefix="$(
    printf '%s\n' "${node_index}" \
      | awk -v arch="${node_arch}" '$2 ~ ("linux-" arch "\\.tar\\.xz$") { sub("-linux-" arch "\\.tar\\.xz$", "", $2); print $2; exit }'
  )"

  if [[ -z "${node_prefix}" ]]; then
    log "Failed to resolve a Node.js v22 Linux package for architecture ${node_arch}"
    exit 1
  fi

  node_tarball="${node_prefix}-linux-${node_arch}.tar.xz"
  node_url="${DEFAULT_NODE_DIST_BASE_URL}/${node_tarball}"

  log "Installing Node.js runtime ${node_prefix} for Linux ${node_arch}"
  mkdir -p "${install_root}"
  rm -rf "${install_root:?}/${node_prefix}-linux-${node_arch}"
  curl -fsSL "${node_url}" | tar -xJf - -C "${install_root}"
  ln -sfn "${install_root}/${node_prefix}-linux-${node_arch}" "${install_link}"

  if ! grep -q '/.local/node-current/bin' "${HOME}/.profile" 2>/dev/null; then
    printf '\nexport PATH="$HOME/.local/node-current/bin:$PATH"\n' >> "${HOME}/.profile"
  fi

  export PATH="${install_link}/bin:${PATH}"
}


ensure_node_runtime() {
  local candidates=(
    "${SAU_NODE_BIN:-}"
    "${HOME}/.local/node-current/bin/node"
    "$(command -v node || true)"
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    if node_bin_is_compatible "${candidate}"; then
      export PATH="$(cd -- "$(dirname -- "${candidate}")" && pwd):${PATH}"
      return 0
    fi
  done

  install_node_runtime
}


write_utf8_requirements() {
  local output_file="$1"
  python3 - "${ROOT_DIR}/requirements.txt" "${output_file}" <<'PY'
from pathlib import Path
import codecs
import sys

source = Path(sys.argv[1])
target = Path(sys.argv[2])
raw = source.read_bytes()
if raw.startswith(codecs.BOM_UTF16_LE) or raw.startswith(codecs.BOM_UTF16_BE):
    text = raw.decode("utf-16")
else:
    text = raw.decode("utf-8")
target.write_text(text, encoding="utf-8")
PY
}


install_python_dependencies() {
  local requirements_tmp
  requirements_tmp="$(mktemp)"
  write_utf8_requirements "${requirements_tmp}"
  log "Installing Python dependencies into .venv"
  export PIP_CONFIG_FILE=/dev/null
  "${ROOT_DIR}/.venv/bin/pip" install --index-url "${DEFAULT_PIP_INDEX_URL}" -r "${requirements_tmp}"
  rm -f "${requirements_tmp}"
}


install_frontend_dependencies() {
  log "Installing sau_frontend dependencies"
  cd "${ROOT_DIR}/sau_frontend"
  if [[ -f package-lock.json ]]; then
    npm ci
    return 0
  fi
  npm install
}


build_frontend() {
  log "Building sau_frontend"
  cd "${ROOT_DIR}/sau_frontend"
  npm run build
}


ensure_openclaw_runtime() {
  if [[ "${OPENCLAW_INSTALL_ON_BOOTSTRAP}" != "1" ]]; then
    return 0
  fi

  mkdir -p "${HOME}/.npm-global"
  export PATH="${HOME}/.npm-global/bin:${PATH}"
  npm config set prefix "${HOME}/.npm-global" >/dev/null 2>&1 || true

  if [[ ! -x "${HOME}/.npm-global/bin/openclaw" ]]; then
    log "Installing OpenClaw CLI"
    npm install -g openclaw@latest
  fi

  if ! grep -q '/.npm-global/bin' "${HOME}/.profile" 2>/dev/null; then
    printf '\nexport PATH="$HOME/.npm-global/bin:$PATH"\n' >> "${HOME}/.profile"
  fi
}


ensure_openclaw_plugins() {
  if [[ "${OPENCLAW_INSTALL_ON_BOOTSTRAP}" != "1" ]]; then
    return 0
  fi

  export PATH="${HOME}/.npm-global/bin:${PATH}"
  if [[ -d "${ROOT_DIR}/openclaw_extensions/omnibull" ]]; then
    log "Installing OpenClaw OmniBull plugin"
    openclaw plugins install -l "${ROOT_DIR}/openclaw_extensions/omnibull"
    openclaw plugins enable omnibull
  fi
  if [[ -d "${ROOT_DIR}/openclaw_extensions/omnidrive" ]]; then
    log "Installing OpenClaw OmniDrive plugin"
    openclaw plugins install -l "${ROOT_DIR}/openclaw_extensions/omnidrive"
    openclaw plugins enable omnidrive
  fi
}


ensure_openclaw_gateway() {
  if [[ "${OPENCLAW_INSTALL_ON_BOOTSTRAP}" != "1" ]]; then
    return 0
  fi

  export PATH="${HOME}/.npm-global/bin:${PATH}"

  python3 - <<'PY'
import json
from pathlib import Path

path = Path.home() / ".openclaw" / "openclaw.json"
path.parent.mkdir(parents=True, exist_ok=True)
if path.exists():
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        data = {}
else:
    data = {}
gateway = data.get("gateway")
if not isinstance(gateway, dict):
    gateway = {}
gateway["mode"] = "local"
data["gateway"] = gateway
path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY

  log "Installing OpenClaw gateway service"
  openclaw gateway install

  if command -v loginctl >/dev/null 2>&1; then
    run_as_root loginctl enable-linger "${USER}" || true
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl --user daemon-reload || true
    systemctl --user enable --now openclaw-gateway.service || true
  fi
}


verify_browser() {
  if command -v chromium >/dev/null 2>&1; then
    log "Using system browser: $(command -v chromium)"
    return 0
  fi
  log "chromium installation was expected but not found"
  exit 1
}


main() {
  ensure_apt
  install_system_packages
  ensure_node_runtime
  ensure_venv
  install_python_dependencies
  install_frontend_dependencies
  build_frontend
  ensure_openclaw_runtime
  ensure_openclaw_plugins
  ensure_openclaw_gateway
  verify_browser
  log "Linux runtime bootstrap completed"
}


main "$@"
