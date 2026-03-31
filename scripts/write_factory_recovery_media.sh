#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

usage() {
  cat <<'EOF'
Usage:
  write_factory_recovery_media.sh <recovery-iso> <target-usb-disk> <factory-bundle-dir>

Examples:
  sudo bash scripts/write_factory_recovery_media.sh /home/sun/Desktop/OmniBull-Recovery.iso /dev/sdb /home/sun/Desktop/omnibull-factory-bundle

Environment overrides:
  OMNIBULL_RECOVERY_LABEL        Label of the bundle partition on the USB (default: OMNIFACTORY)
  OMNIBULL_RECOVERY_BUNDLE_FS    Filesystem for the bundle partition: exfat or ext4 (default: exfat)
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "missing required command: ${cmd}" >&2
    exit 1
  fi
}

part_path() {
  local disk="$1"
  local partnum="$2"
  if [[ "${disk}" =~ (nvme|mmcblk|loop)[0-9]+$ ]]; then
    printf '%sp%s\n' "${disk}" "${partnum}"
    return 0
  fi
  printf '%s%s\n' "${disk}" "${partnum}"
}

unmount_disk_partitions() {
  local disk="$1"
  local part
  while read -r part; do
    [[ -n "${part}" ]] || continue
    umount -R "${part}" 2>/dev/null || true
  done < <(lsblk -ln -o PATH "${disk}" | tail -n +2)
}

find_last_free_start_mib() {
  local disk="$1"
  parted -sm "${disk}" unit MiB print free \
    | awk -F: '$1 == "" && $5 == "free;" { gsub("MiB", "", $2); start = $2 } END { if (start == "") exit 1; print start }'
}

main() {
  if [[ $# -ne 3 ]]; then
    usage >&2
    exit 1
  fi

  local recovery_iso="$1"
  local target_usb="$2"
  local bundle_dir="$3"
  local bundle_label="${OMNIBULL_RECOVERY_LABEL:-OMNIFACTORY}"
  local bundle_fs="${OMNIBULL_RECOVERY_BUNDLE_FS:-exfat}"
  local work_root="${OMNIBULL_RECOVERY_WORK_DIR:-/var/tmp/omnibull-recovery-media}"
  local mount_root="${work_root}/bundle-mount"
  local restart_udisks=0

  if [[ ! -f "${recovery_iso}" ]]; then
    echo "recovery iso not found: ${recovery_iso}" >&2
    exit 1
  fi
  if [[ ! -b "${target_usb}" ]]; then
    echo "target USB is not a block device: ${target_usb}" >&2
    exit 1
  fi
  if [[ ! -d "${bundle_dir}" ]]; then
    echo "factory bundle directory not found: ${bundle_dir}" >&2
    exit 1
  fi

  require_cmd dd
  require_cmd lsblk
  require_cmd parted
  require_cmd partprobe
  require_cmd udevadm
  require_cmd rsync
  case "${bundle_fs}" in
    exfat)
      require_cmd mkfs.exfat
      ;;
    ext4)
      require_cmd mkfs.ext4
      ;;
    *)
      echo "unsupported bundle filesystem: ${bundle_fs}" >&2
      exit 1
      ;;
  esac

  rm -rf "${work_root}"
  mkdir -p "${mount_root}"
  trap '
    umount -R "'"${mount_root}"'" 2>/dev/null || true
    if [[ "'"${restart_udisks}"'" == "1" ]]; then
      systemctl start udisks2.service >/dev/null 2>&1 || true
    fi
    rm -rf "'"${work_root}"'"
  ' EXIT

  systemctl stop udisks2.service >/dev/null 2>&1 || true
  restart_udisks=1
  unmount_disk_partitions "${target_usb}"

  echo "Writing ${recovery_iso} to ${target_usb}"
  dd if="${recovery_iso}" of="${target_usb}" bs=16M oflag=direct status=progress conv=fsync
  sync
  sleep 2
  unmount_disk_partitions "${target_usb}"
  partprobe "${target_usb}" || true
  blockdev --rereadpt "${target_usb}" || true
  udevadm settle || true

  local free_start
  free_start="$(find_last_free_start_mib "${target_usb}")"
  echo "Creating ${bundle_label} partition from ${free_start} MiB"
  parted -s "${target_usb}" mkpart "${bundle_label}" ext4 "${free_start}MiB" 100%
  partprobe "${target_usb}" || true
  udevadm settle || true

  local bundle_part
  bundle_part="$(part_path "${target_usb}" 3)"
  case "${bundle_fs}" in
    exfat)
      mkfs.exfat -n "${bundle_label}" "${bundle_part}" >/dev/null
      ;;
    ext4)
      mkfs.ext4 -F -L "${bundle_label}" "${bundle_part}" >/dev/null
      ;;
  esac

  mount "${bundle_part}" "${mount_root}"
  mkdir -p "${mount_root}/$(basename "${bundle_dir}")"
  rsync -aH --info=progress2 "${bundle_dir}/" "${mount_root}/$(basename "${bundle_dir}")/"
  cat > "${mount_root}/START_HERE.txt" <<EOF
OmniBull Factory Recovery USB

Boot from this USB and keep the default menu entry:
  OmniBull Factory Restore

Bundle directory:
  $(basename "${bundle_dir}")

Manual fallback in a Linux rescue shell:
  sudo bash /mnt/${bundle_label}/$(basename "${bundle_dir}")/restore.sh /dev/sda
EOF
  sync
  umount "${mount_root}"
  systemctl start udisks2.service >/dev/null 2>&1 || true
  restart_udisks=0

  lsblk -o NAME,PATH,SIZE,FSTYPE,LABEL,MOUNTPOINT "${target_usb}"
  echo "Recovery USB is ready: ${target_usb}"
}

main "$@"
