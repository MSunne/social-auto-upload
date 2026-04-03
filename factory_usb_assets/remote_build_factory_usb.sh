#!/bin/bash
set -euo pipefail

WORKDIR="${WORKDIR:-$HOME/factory-usb-work}"
USB_PART="${USB_PART:-/dev/sdb1}"
ISO_NAME="clonezilla-live-3.3.1-35-amd64.iso"
ISO_URL="https://clonezilla.nchc.org.tw/clonezilla-live/download/stable/${ISO_NAME}"
ISO_SHA256="ac4f88c8795a917e3d3fc1a3e52d095f35fe531d459cf853cd3e2c7731043fec"

cd "$WORKDIR"

if [ -f "$ISO_NAME" ] && ! printf '%s  %s\n' "$ISO_SHA256" "$ISO_NAME" | sha256sum -c - >/dev/null 2>&1; then
  rm -f "$ISO_NAME"
fi

if [ ! -f "$ISO_NAME" ]; then
  wget -O "$ISO_NAME" "$ISO_URL"
fi

printf '%s  %s\n' "$ISO_SHA256" "$ISO_NAME" | sha256sum -c -

ocs-live-dev -f -g en_US.UTF-8 -k NONE -s -d "$USB_PART" -m "$WORKDIR/custom-ocs" -j "$WORKDIR/$ISO_NAME"

tmp_mount="$(mktemp -d /tmp/factoryusb.XXXXXX)"
trap 'umount "$tmp_mount" >/dev/null 2>&1 || true; rmdir "$tmp_mount" >/dev/null 2>&1 || true' EXIT

mount "$USB_PART" "$tmp_mount"
mkdir -p "$tmp_mount/partimag" "$tmp_mount/logs"
cp "$WORKDIR/README-supplier.txt" "$tmp_mount/README-supplier.txt"
sync

echo "=== factory usb files ==="
find "$tmp_mount" -maxdepth 3 \( \
  -name custom-ocs -o \
  -name README-supplier.txt -o \
  -name grub.cfg -o \
  -name syslinux.cfg -o \
  -name isolinux.cfg -o \
  -path "*/EFI/*" \
\) | sort

umount "$tmp_mount"
rmdir "$tmp_mount"
trap - EXIT

lsblk -o NAME,PATH,SIZE,RM,RO,TYPE,FSTYPE,MOUNTPOINT,TRAN,HOTPLUG,LABEL "$(dirname "$USB_PART")"
