#!/bin/bash
set -euo pipefail

FILE="${1:-clonezilla-live-3.3.1-35-amd64.zip}"
BASE="https://downloads.sourceforge.net/project/clonezilla/clonezilla_live_stable/3.3.1-35/${FILE}"
MIRRORS=(nchc jaist versaweb pilotfiber netactuate phoenixnap freefr netix gigenet)

for mirror in "${MIRRORS[@]}"; do
  url="${BASE}?use_mirror=${mirror}"
  speed="$(curl -L --fail --silent --show-error --range 0-1048575 -o /dev/null -w '%{speed_download}' "$url" || echo 0)"
  printf '%-12s %s\n' "$mirror" "$speed"
done
