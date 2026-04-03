#!/bin/bash
set -euo pipefail

url="$1"
curl -L --fail --connect-timeout 10 --max-time 30 \
  --range 0-1048575 -o /dev/null \
  -w 'http=%{http_code} speed=%{speed_download} effective=%{url_effective}\n' \
  "$url"
