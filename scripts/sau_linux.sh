#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

RUNTIME_DIR="${SAU_RUNTIME_DIR:-${ROOT_DIR}/runtime}"
LOG_DIR="${SAU_LOG_DIR:-${ROOT_DIR}/logs}"

BACKEND_PID_FILE="${RUNTIME_DIR}/sau_backend.pid"
FRONTEND_PID_FILE="${RUNTIME_DIR}/sau_frontend.pid"
LAUNCHER_PID_FILE="${RUNTIME_DIR}/sau_stack.pid"

BACKEND_LOG_FILE="${LOG_DIR}/sau_backend.start.log"
FRONTEND_LOG_FILE="${LOG_DIR}/sau_frontend.start.log"
LAUNCHER_LOG_FILE="${LOG_DIR}/sau_stack.log"

BACKEND_PORT="${SAU_BACKEND_PORT:-5409}"
FRONTEND_PORT="${SAU_FRONTEND_PORT:-5173}"
FRONTEND_HOST="${SAU_FRONTEND_HOST:-0.0.0.0}"

mkdir -p "${RUNTIME_DIR}" "${LOG_DIR}"


log() {
  local level="$1"
  shift
  printf '[%s] [%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "${level}" "$*" | tee -a "${LAUNCHER_LOG_FILE}"
}


pick_python_bin() {
  local configured="${SAU_PYTHON_BIN:-}"
  local candidates=(
    "${configured}"
    "${ROOT_DIR}/.venv/bin/python"
    "${ROOT_DIR}/venv/bin/python"
    "${ROOT_DIR}/env/bin/python"
    "$(command -v python3 || true)"
    "$(command -v python || true)"
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    if [[ -n "${candidate}" && -x "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}


pick_npm_bin() {
  local configured="${SAU_NPM_BIN:-}"
  local candidates=(
    "${configured}"
    "$(command -v npm || true)"
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    if [[ -n "${candidate}" && -x "${candidate}" ]]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done
  return 1
}


is_pid_running() {
  local pid="$1"
  [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null
}


read_pid() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]]; then
    tr -d ' \n\r\t' < "${pid_file}"
  fi
}


is_running_from_pid_file() {
  local pid_file="$1"
  local pid
  pid="$(read_pid "${pid_file}")"
  is_pid_running "${pid}"
}


write_pid() {
  local pid_file="$1"
  local pid="$2"
  printf '%s\n' "${pid}" > "${pid_file}"
}


clear_pid_file_if_stale() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]] && ! is_running_from_pid_file "${pid_file}"; then
    rm -f "${pid_file}"
  fi
}


stop_process_from_file() {
  local name="$1"
  local pid_file="$2"
  local pid

  pid="$(read_pid "${pid_file}")"
  if [[ -z "${pid}" ]]; then
    rm -f "${pid_file}"
    return 0
  fi
  if ! is_pid_running "${pid}"; then
    rm -f "${pid_file}"
    return 0
  fi

  log INFO "Stopping ${name} pid=${pid}"
  kill "${pid}" 2>/dev/null || true
  for _ in {1..20}; do
    if ! is_pid_running "${pid}"; then
      rm -f "${pid_file}"
      return 0
    fi
    sleep 1
  done

  log WARNING "${name} did not exit in time, force killing pid=${pid}"
  kill -9 "${pid}" 2>/dev/null || true
  rm -f "${pid_file}"
}


start_backend() {
  local python_bin="$1"
  if is_running_from_pid_file "${BACKEND_PID_FILE}"; then
    log INFO "SAU backend already running pid=$(read_pid "${BACKEND_PID_FILE}")"
    return 0
  fi

  log INFO "Starting SAU backend on port ${BACKEND_PORT}"
  (
    cd "${ROOT_DIR}"
    export PYTHONUNBUFFERED=1
    export SAU_BACKEND_PORT="${BACKEND_PORT}"
    exec "${python_bin}" "${ROOT_DIR}/sau_backend.py"
  ) >> "${BACKEND_LOG_FILE}" 2>&1 &
  write_pid "${BACKEND_PID_FILE}" "$!"
}


start_frontend() {
  local npm_bin="$1"
  if is_running_from_pid_file "${FRONTEND_PID_FILE}"; then
    log INFO "SAU frontend already running pid=$(read_pid "${FRONTEND_PID_FILE}")"
    return 0
  fi

  log INFO "Starting SAU frontend on ${FRONTEND_HOST}:${FRONTEND_PORT}"
  (
    cd "${ROOT_DIR}/sau_frontend"
    export CI=1
    exec "${npm_bin}" run dev -- --host "${FRONTEND_HOST}" --port "${FRONTEND_PORT}" --strictPort
  ) >> "${FRONTEND_LOG_FILE}" 2>&1 &
  write_pid "${FRONTEND_PID_FILE}" "$!"
}


run_stack() {
  local python_bin
  local npm_bin

  python_bin="$(pick_python_bin)" || {
    log ERROR "Python runtime not found. Set SAU_PYTHON_BIN or create .venv/venv."
    exit 1
  }
  npm_bin="$(pick_npm_bin)" || {
    log ERROR "npm not found. Set SAU_NPM_BIN or install Node.js."
    exit 1
  }

  if [[ ! -f "${ROOT_DIR}/conf.py" ]]; then
    log ERROR "conf.py not found at ${ROOT_DIR}/conf.py"
    exit 1
  fi
  if [[ ! -d "${ROOT_DIR}/sau_frontend" ]]; then
    log ERROR "sau_frontend directory not found at ${ROOT_DIR}/sau_frontend"
    exit 1
  fi

  clear_pid_file_if_stale "${BACKEND_PID_FILE}"
  clear_pid_file_if_stale "${FRONTEND_PID_FILE}"

  start_backend "${python_bin}"
  start_frontend "${npm_bin}"

  write_pid "${LAUNCHER_PID_FILE}" "$$"
  log INFO "SAU stack started launcher_pid=$$ backend_pid=$(read_pid "${BACKEND_PID_FILE}") frontend_pid=$(read_pid "${FRONTEND_PID_FILE}")"

  cleanup() {
    log INFO "Stopping SAU stack"
    stop_process_from_file "SAU frontend" "${FRONTEND_PID_FILE}"
    stop_process_from_file "SAU backend" "${BACKEND_PID_FILE}"
    rm -f "${LAUNCHER_PID_FILE}"
  }

  trap cleanup EXIT INT TERM

  local backend_pid
  local frontend_pid
  backend_pid="$(read_pid "${BACKEND_PID_FILE}")"
  frontend_pid="$(read_pid "${FRONTEND_PID_FILE}")"

  while true; do
    if ! is_pid_running "${backend_pid}"; then
      log ERROR "SAU backend exited unexpectedly pid=${backend_pid}"
      exit 1
    fi
    if ! is_pid_running "${frontend_pid}"; then
      log ERROR "SAU frontend exited unexpectedly pid=${frontend_pid}"
      exit 1
    fi
    sleep 2
  done
}


start_daemon() {
  clear_pid_file_if_stale "${LAUNCHER_PID_FILE}"
  if is_running_from_pid_file "${LAUNCHER_PID_FILE}"; then
    log INFO "SAU stack launcher already running pid=$(read_pid "${LAUNCHER_PID_FILE}")"
    exit 0
  fi

  nohup bash "${SCRIPT_DIR}/sau_linux.sh" run >> "${LAUNCHER_LOG_FILE}" 2>&1 &
  disown || true
  sleep 2

  if is_running_from_pid_file "${LAUNCHER_PID_FILE}"; then
    log INFO "SAU stack launcher started pid=$(read_pid "${LAUNCHER_PID_FILE}")"
    exit 0
  fi

  log ERROR "SAU stack launcher failed to start. Check ${LAUNCHER_LOG_FILE}"
  exit 1
}


stop_daemon() {
  stop_process_from_file "SAU stack launcher" "${LAUNCHER_PID_FILE}"
  stop_process_from_file "SAU frontend" "${FRONTEND_PID_FILE}"
  stop_process_from_file "SAU backend" "${BACKEND_PID_FILE}"
}


status_stack() {
  clear_pid_file_if_stale "${LAUNCHER_PID_FILE}"
  clear_pid_file_if_stale "${BACKEND_PID_FILE}"
  clear_pid_file_if_stale "${FRONTEND_PID_FILE}"

  printf 'launcher: %s\n' "$(read_pid "${LAUNCHER_PID_FILE}")"
  printf 'backend : %s\n' "$(read_pid "${BACKEND_PID_FILE}")"
  printf 'frontend: %s\n' "$(read_pid "${FRONTEND_PID_FILE}")"
  printf 'backend log : %s\n' "${BACKEND_LOG_FILE}"
  printf 'frontend log: %s\n' "${FRONTEND_LOG_FILE}"
  printf 'launcher log: %s\n' "${LAUNCHER_LOG_FILE}"
}


usage() {
  cat <<EOF
Usage: $(basename "$0") <run|start|stop|restart|status>

Commands:
  run      Run SAU backend and sau_frontend in the foreground.
  start    Start the stack in the background.
  stop     Stop the stack and child processes.
  restart  Restart the stack.
  status   Print current pid and log file locations.

Environment overrides:
  SAU_PYTHON_BIN      Explicit python executable.
  SAU_NPM_BIN         Explicit npm executable.
  SAU_BACKEND_PORT    Backend port, default 5409.
  SAU_FRONTEND_HOST   Frontend bind host, default 0.0.0.0.
  SAU_FRONTEND_PORT   Frontend port, default 5173.
  SAU_RUNTIME_DIR     PID directory, default ${ROOT_DIR}/runtime.
  SAU_LOG_DIR         Log directory, default ${ROOT_DIR}/logs.
EOF
}


main() {
  local command="${1:-run}"
  case "${command}" in
    run)
      run_stack
      ;;
    start)
      start_daemon
      ;;
    stop)
      stop_daemon
      ;;
    restart)
      stop_daemon
      start_daemon
      ;;
    status)
      status_stack
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}


main "${1:-run}"
