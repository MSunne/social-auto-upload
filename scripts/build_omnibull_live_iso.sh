#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo "$0" "$@"
fi

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
HOSTNAME_VALUE="${OMNIBULL_ISO_HOSTNAME:-omnibull}"
LABEL="${OMNIBULL_ISO_LABEL:-OMNIBULL_LIVE}"
USER_HOME="${SUDO_USER:+/home/${SUDO_USER}}"
if [[ -z "${USER_HOME}" || ! -d "${USER_HOME}" ]]; then
  USER_HOME="/root"
fi

DESKTOP_DIR="${OMNIBULL_ISO_OUTPUT_DIR:-${USER_HOME}/Desktop}"
BUILD_ROOT="${OMNIBULL_ISO_BUILD_ROOT:-${DESKTOP_DIR}/omnibull-live-iso-build}"
OUTPUT_ISO="${OMNIBULL_ISO_OUTPUT_PATH:-${DESKTOP_DIR}/OmniBull-Deepin25-${TIMESTAMP}.iso}"
OUTPUT_SUM="${OUTPUT_ISO}.sha256"
KERNEL_VERSION="${OMNIBULL_ISO_KERNEL_VERSION:-$(uname -r)}"

mkdir -p "${DESKTOP_DIR}"
rm -rf "${BUILD_ROOT}"
mkdir -p "${BUILD_ROOT}/iso-root/live" "${BUILD_ROOT}/iso-root/boot/grub"

apt-get update
apt-get install -y xorriso squashfs-tools grub-pc-bin grub-efi-amd64-bin live-boot-initramfs-tools

update-initramfs -u -k "${KERNEL_VERSION}"

INITRD_CONTENTS_LIST="${BUILD_ROOT}/initrd.contents"
lsinitramfs "/boot/initrd.img-${KERNEL_VERSION}" > "${INITRD_CONTENTS_LIST}"

if ! grep -qE 'live-boot|scripts/live' "${INITRD_CONTENTS_LIST}"; then
  echo "live-boot scripts were not found inside /boot/initrd.img-${KERNEL_VERSION}" >&2
  exit 1
fi

cp "/boot/vmlinuz-${KERNEL_VERSION}" "${BUILD_ROOT}/iso-root/live/vmlinuz"
cp "/boot/initrd.img-${KERNEL_VERSION}" "${BUILD_ROOT}/iso-root/live/initrd.img"

cat > "${BUILD_ROOT}/exclude.list" <<EOF
boot
proc
sys
dev
run
tmp
mnt
media
lost+found
swapfile
var/tmp
var/cache/apt/archives
var/lib/systemd/coredump
home/*/.cache
home/*/.npm/_cacache
home/*/.local/share/Trash
home/*/Desktop/omnibull-live-iso-build
home/*/Desktop/OmniBull-Deepin25-*.iso
EOF

mksquashfs / "${BUILD_ROOT}/iso-root/live/filesystem.squashfs" \
  -wildcards \
  -ef "${BUILD_ROOT}/exclude.list" \
  -comp xz \
  -b 1048576 \
  -noappend

printf '%s\n' "$(stat -c '%s' "${BUILD_ROOT}/iso-root/live/filesystem.squashfs")" > "${BUILD_ROOT}/iso-root/live/filesystem.size"

cat > "${BUILD_ROOT}/iso-root/boot/grub/grub.cfg" <<EOF
set default=0
set timeout=5

menuentry "OmniBull Live" {
    linux /live/vmlinuz boot=live components quiet splash hostname=${HOSTNAME_VALUE} username=$(basename "${USER_HOME}")
    initrd /live/initrd.img
}

menuentry "OmniBull Live (failsafe)" {
    linux /live/vmlinuz boot=live components nomodeset nosplash hostname=${HOSTNAME_VALUE} username=$(basename "${USER_HOME}")
    initrd /live/initrd.img
}
EOF

grub-mkrescue -o "${OUTPUT_ISO}" "${BUILD_ROOT}/iso-root"
sha256sum "${OUTPUT_ISO}" > "${OUTPUT_SUM}"

if [[ -n "${SUDO_USER:-}" ]]; then
  chown -R "${SUDO_USER}:${SUDO_USER}" "${BUILD_ROOT}" "${OUTPUT_ISO}" "${OUTPUT_SUM}"
fi

echo "ISO built at ${OUTPUT_ISO}"
echo "SHA256 saved at ${OUTPUT_SUM}"
