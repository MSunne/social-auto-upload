#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="${OMNIBULL_FACTORY_BUNDLE_DIR:-${SCRIPT_DIR}}"
TARGET_DISK="${OMNIBULL_FACTORY_TARGET_DISK:-${1:-/dev/sda}}"
WORK_DIR="${OMNIBULL_FACTORY_WORK_DIR:-/var/tmp/omnibull-factory-restore}"
TARGET_ROOT="${WORK_DIR}/target-root"
PRESERVE_DEVICE_IDENTITY="${OMNIBULL_FACTORY_PRESERVE_DEVICE_IDENTITY:-0}"

ARCHIVES_DIR="${BUNDLE_DIR}/archives"
META_DIR="${BUNDLE_DIR}/meta"
PROVISIONING_DIR="${BUNDLE_DIR}/provisioning"
CHECKSUM_FILE="${BUNDLE_DIR}/SHA256SUMS"
PARTITION_MAP_FILE="${META_DIR}/partition-map.tsv"
LAYOUT_FILE="${META_DIR}/disk-layout.sfdisk"

declare -A PART_PATHS=()
declare -A PART_OLD_UUIDS=()
declare -A PART_NEW_UUIDS=()
declare -A PART_FS=()
declare -A PART_MOUNTS=()
declare -A PART_LABELS=()


log() {
  printf '[%s] [factory-restore] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}


require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    log "missing required command: ${cmd}"
    exit 1
  fi
}


ensure_requirements() {
  require_cmd tar
  require_cmd zstd
  require_cmd blkid
  require_cmd lsblk
  require_cmd sfdisk
  require_cmd mkfs.ext4
  require_cmd mkfs.vfat
  require_cmd mkswap
  require_cmd grub-install
  require_cmd update-grub
}


assert_bundle_layout() {
  local path
  for path in \
    "${ARCHIVES_DIR}/efi.tar.zst" \
    "${ARCHIVES_DIR}/boot.tar.zst" \
    "${ARCHIVES_DIR}/root.tar.zst" \
    "${PARTITION_MAP_FILE}" \
    "${LAYOUT_FILE}"
  do
    if [[ ! -e "${path}" ]]; then
      log "bundle is missing required file: ${path}"
      exit 1
    fi
  done
}


verify_checksums_if_present() {
  if [[ ! -f "${CHECKSUM_FILE}" ]]; then
    return 0
  fi
  log "verifying bundle checksums"
  (
    cd "${BUNDLE_DIR}"
    sha256sum -c "${CHECKSUM_FILE}"
  )
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


stop_target_mounts() {
  local item
  while read -r item; do
    [[ -n "${item}" ]] || continue
    swapoff "${item}" 2>/dev/null || true
    umount -R "${item}" 2>/dev/null || true
  done < <(lsblk -ln -o PATH "${TARGET_DISK}" | tail -n +2)
}


sanitize_layout() {
  local output_file="$1"
  sed -E \
    -e '/^label-id:/d' \
    -e 's/, *uuid=[^, ]+//g' \
    -e 's/, *attrs=[^, ]+//g' \
    "${LAYOUT_FILE}" > "${output_file}"
}


apply_partition_layout() {
  local sanitized_layout
  sanitized_layout="$(mktemp)"
  sanitize_layout "${sanitized_layout}"
  stop_target_mounts
  sfdisk --delete "${TARGET_DISK}" >/dev/null 2>&1 || true
  sfdisk "${TARGET_DISK}" < "${sanitized_layout}"
  rm -f "${sanitized_layout}"
  partprobe "${TARGET_DISK}" || true
  udevadm settle || true
}


load_partition_map() {
  local role partnum fstype mountpoint label uuid partuuid
  while IFS=$'\t' read -r role partnum fstype mountpoint label uuid partuuid; do
    if [[ "${role}" == "role" ]]; then
      continue
    fi
    PART_PATHS["${role}"]="$(part_path "${TARGET_DISK}" "${partnum}")"
    PART_OLD_UUIDS["${role}"]="${uuid}"
    PART_FS["${role}"]="${fstype}"
    PART_MOUNTS["${role}"]="${mountpoint}"
    PART_LABELS["${role}"]="${label}"
  done < "${PARTITION_MAP_FILE}"
}


format_partitions() {
  local role path label
  for role in efi boot root persistent; do
    path="${PART_PATHS[${role}]:-}"
    [[ -n "${path}" ]] || continue
    label="${PART_LABELS[${role}]:-${role}}"
    case "${role}" in
      efi)
        mkfs.vfat -F 32 -n "${label:-EFI}" "${path}"
        ;;
      *)
        mkfs.ext4 -F -L "${label:-${role}}" "${path}"
        ;;
    esac
    PART_NEW_UUIDS["${role}"]="$(blkid -s UUID -o value "${path}")"
  done

  if [[ -n "${PART_PATHS[swap]:-}" ]]; then
    mkswap -L "${PART_LABELS[swap]:-SWAP}" "${PART_PATHS[swap]}"
    PART_NEW_UUIDS["swap"]="$(blkid -s UUID -o value "${PART_PATHS[swap]}")"
  fi
}


mount_target_layout() {
  rm -rf "${WORK_DIR}"
  mkdir -p "${TARGET_ROOT}"

  mount "${PART_PATHS[root]}" "${TARGET_ROOT}"
  mkdir -p "${TARGET_ROOT}/boot" "${TARGET_ROOT}/boot/efi"
  mount "${PART_PATHS[boot]}" "${TARGET_ROOT}/boot"
  mount "${PART_PATHS[efi]}" "${TARGET_ROOT}/boot/efi"

  if [[ -n "${PART_PATHS[persistent]:-}" ]]; then
    mkdir -p "${TARGET_ROOT}/persistent"
    mount "${PART_PATHS[persistent]}" "${TARGET_ROOT}/persistent"
  fi
}


extract_archive() {
  local archive_path="$1"
  local destination="$2"
  log "extracting $(basename "${archive_path}") into ${destination}"
  mkdir -p "${destination}"
  tar \
    --numeric-owner \
    --acls \
    --xattrs \
    -I 'zstd -T0 -d' \
    -xpf "${archive_path}" \
    -C "${destination}"
}


extract_archives() {
  extract_archive "${ARCHIVES_DIR}/root.tar.zst" "${TARGET_ROOT}"
  extract_archive "${ARCHIVES_DIR}/boot.tar.zst" "${TARGET_ROOT}/boot"
  extract_archive "${ARCHIVES_DIR}/efi.tar.zst" "${TARGET_ROOT}/boot/efi"
  if [[ -f "${ARCHIVES_DIR}/persistent.tar.zst" && -d "${TARGET_ROOT}/persistent" ]]; then
    extract_archive "${ARCHIVES_DIR}/persistent.tar.zst" "${TARGET_ROOT}/persistent"
  fi
}


replace_uuid_in_file() {
  local file_path="$1"
  local role old_uuid new_uuid
  for role in "${!PART_OLD_UUIDS[@]}"; do
    old_uuid="${PART_OLD_UUIDS[${role}]}"
    new_uuid="${PART_NEW_UUIDS[${role}]:-}"
    if [[ -z "${old_uuid}" || -z "${new_uuid}" ]]; then
      continue
    fi
    sed -i "s/${old_uuid}/${new_uuid}/g" "${file_path}"
  done
}


patch_fstab_candidates() {
  local file_path
  while read -r file_path; do
    [[ -n "${file_path}" ]] || continue
    replace_uuid_in_file "${file_path}"
  done < <(find "${TARGET_ROOT}" "${TARGET_ROOT}/persistent" -type f -path '*/etc/fstab' 2>/dev/null | sort -u)
}


reset_cloned_identity() {
  if [[ "${PRESERVE_DEVICE_IDENTITY}" == "1" ]]; then
    return 0
  fi

  rm -f "${TARGET_ROOT}/etc/omnibull/device.json" 2>/dev/null || true
  find "${TARGET_ROOT}" "${TARGET_ROOT}/persistent" -type f -name 'device.identity.json' -delete 2>/dev/null || true

  : > "${TARGET_ROOT}/etc/machine-id"
  rm -f "${TARGET_ROOT}/var/lib/dbus/machine-id" 2>/dev/null || true
  rm -f "${TARGET_ROOT}/persistent/var/lib/dbus/machine-id" 2>/dev/null || true
}


apply_provisioning() {
  if [[ -d "${PROVISIONING_DIR}/rootfs" ]]; then
    log "applying provisioning/rootfs overrides"
    tar -C "${PROVISIONING_DIR}/rootfs" -cpf - . | tar -C "${TARGET_ROOT}" -xpf -
  fi
}


run_in_chroot() {
  local command="$1"
  chroot "${TARGET_ROOT}" /bin/bash -lc "${command}"
}


bind_chroot_mounts() {
  mkdir -p \
    "${TARGET_ROOT}/dev" \
    "${TARGET_ROOT}/proc" \
    "${TARGET_ROOT}/sys" \
    "${TARGET_ROOT}/run" \
    "${TARGET_ROOT}/tmp"
  mount --bind /dev "${TARGET_ROOT}/dev"
  mount --bind /proc "${TARGET_ROOT}/proc"
  mount --bind /sys "${TARGET_ROOT}/sys"
  mount --bind /run "${TARGET_ROOT}/run"
}


unbind_chroot_mounts() {
  umount -R "${TARGET_ROOT}/run" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/sys" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/proc" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/dev" 2>/dev/null || true
}


reinstall_bootloader() {
  bind_chroot_mounts
  trap unbind_chroot_mounts RETURN

  local efi_mount="/boot/efi"
  if [[ -d "${TARGET_ROOT}/sys/firmware/efi" || -d /sys/firmware/efi ]]; then
    run_in_chroot "grub-install --target=x86_64-efi --efi-directory=${efi_mount} --bootloader-id=deepin --recheck"
  else
    run_in_chroot "grub-install ${TARGET_DISK}"
  fi
  run_in_chroot "update-grub"

  if [[ -x "${PROVISIONING_DIR}/post-restore.sh" ]]; then
    cp "${PROVISIONING_DIR}/post-restore.sh" "${TARGET_ROOT}/tmp/post-restore.sh"
    chmod +x "${TARGET_ROOT}/tmp/post-restore.sh"
    run_in_chroot "/tmp/post-restore.sh"
    rm -f "${TARGET_ROOT}/tmp/post-restore.sh"
  fi

  trap - RETURN
  unbind_chroot_mounts
}


cleanup_mounts() {
  swapoff "${PART_PATHS[swap]:-}" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/boot/efi" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/boot" 2>/dev/null || true
  umount -R "${TARGET_ROOT}/persistent" 2>/dev/null || true
  umount -R "${TARGET_ROOT}" 2>/dev/null || true
}


main() {
  ensure_requirements
  assert_bundle_layout
  verify_checksums_if_present
  load_partition_map
  apply_partition_layout
  format_partitions
  mount_target_layout
  trap cleanup_mounts EXIT
  extract_archives
  patch_fstab_candidates
  reset_cloned_identity
  apply_provisioning
  reinstall_bootloader
  trap - EXIT
  cleanup_mounts
  log "factory restore completed for ${TARGET_DISK}"
}


main "$@"
