#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

FFMPEG_BIN="${OMNIDRIVE_AI_VIDEO_FFMPEG_PATH:-$(command -v ffmpeg || true)}"
FFPROBE_BIN="$(command -v ffprobe || true)"
FILTER="scale='max(2,trunc(iw*0.98/2)*2)':'max(2,trunc(ih*0.98/2)*2)',fps=30"
FPS=30
CRF=20

usage() {
  cat <<'EOF'
Usage:
  scripts/test_ffmpeg_standardize.sh [input_video] [output_video]

Behavior:
  - no input_video: generate a 1s synthetic sample and standardize it
  - input_video only: write <input_basename>.standardized.mp4 next to the input
  - input_video + output_video: write to the provided output path
EOF
}

log() {
  printf '[ffmpeg-standardize] %s\n' "$*"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -z "${FFMPEG_BIN}" || ! -x "${FFMPEG_BIN}" ]]; then
  printf 'ffmpeg not found. Install it first, for example: brew install ffmpeg\n' >&2
  exit 1
fi

if [[ -z "${FFPROBE_BIN}" || ! -x "${FFPROBE_BIN}" ]]; then
  printf 'ffprobe not found. It is usually bundled with ffmpeg.\n' >&2
  exit 1
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/omnidrive-ffmpeg-standardize.XXXXXX")"
trap 'rm -rf "${tmp_dir}"' EXIT

input_path="${1:-}"
if [[ -z "${input_path}" ]]; then
  input_path="${tmp_dir}/synthetic-input.mp4"
  log "Generating synthetic input at ${input_path}"
  "${FFMPEG_BIN}" -y \
    -f lavfi -i testsrc=duration=1:size=640x360:rate=25 \
    -f lavfi -i sine=frequency=1000:duration=1 \
    -c:v libx264 \
    -pix_fmt yuv420p \
    -c:a aac \
    -shortest \
    "${input_path}" \
    >/dev/null 2>&1
else
  if [[ ! -f "${input_path}" ]]; then
    printf 'input video does not exist: %s\n' "${input_path}" >&2
    exit 1
  fi
  input_path="$(cd -- "$(dirname -- "${input_path}")" && pwd)/$(basename -- "${input_path}")"
fi

if [[ -n "${2:-}" ]]; then
  output_path="${2}"
  mkdir -p "$(dirname -- "${output_path}")"
  output_path="$(cd -- "$(dirname -- "${output_path}")" && pwd)/$(basename -- "${output_path}")"
else
  input_dir="$(dirname -- "${input_path}")"
  input_name="$(basename -- "${input_path}")"
  input_stem="${input_name%.*}"
  output_path="${input_dir}/${input_stem}.standardized.mp4"
fi

log "Using ffmpeg: ${FFMPEG_BIN}"
log "Input: ${input_path}"
log "Output: ${output_path}"

"${FFMPEG_BIN}" -y \
  -i "${input_path}" \
  -map 0:v:0 \
  -map 0:a? \
  -vf "${FILTER}" \
  -c:v libx264 \
  -preset medium \
  -crf "${CRF}" \
  -pix_fmt yuv420p \
  -r "${FPS}" \
  -c:a aac \
  -b:a 128k \
  -movflags +faststart \
  "${output_path}"

log "Output stream info:"
"${FFPROBE_BIN}" -v error -select_streams v:0 \
  -show_entries stream=codec_name,width,height,pix_fmt,r_frame_rate \
  -of default=noprint_wrappers=1 "${output_path}"
"${FFPROBE_BIN}" -v error -select_streams a:0 \
  -show_entries stream=codec_name,sample_rate \
  -of default=noprint_wrappers=1 "${output_path}"

log "Smoke test completed"
log "Project env ffmpeg path: ${ROOT_DIR}/omnidrive_cloud/.env -> ${FFMPEG_BIN}"
