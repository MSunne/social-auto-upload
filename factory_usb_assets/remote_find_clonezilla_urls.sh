#!/bin/bash
set -euo pipefail

page="$(curl -L -s 'https://clonezilla.nchc.org.tw/clonezilla-live/download/download.php?branch=stable')"
printf '%s' "$page" |
  grep -Eo 'https?://[^"[:space:]]+clonezilla-live-3\.3\.1-35-amd64\.(iso|zip)' |
  sort -u
