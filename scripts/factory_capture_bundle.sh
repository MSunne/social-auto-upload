#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
WORK_USER="${SUDO_USER:-root}"
USER_HOME="$(getent passwd "${WORK_USER}" | cut -d: -f6)"
if [[ -z "${USER_HOME}" || ! -d "${USER_HOME}" ]]; then
  USER_HOME="/root"
fi

BUNDLE_NAME="${OMNIBULL_FACTORY_BUNDLE_NAME:-omnibull-factory-bundle-${TIMESTAMP}}"
OUTPUT_PARENT="${OMNIBULL_FACTORY_OUTPUT_DIR:-${USER_HOME}/Desktop}"
STAGING_DIR="${OMNIBULL_FACTORY_STAGING_DIR:-/var/tmp/${BUNDLE_NAME}}"
FINAL_DIR="${OUTPUT_PARENT%/}/${BUNDLE_NAME}"
SOURCE_DISK="${OMNIBULL_FACTORY_SOURCE_DISK:-}"
STOP_SERVICES_RAW="${OMNIBULL_FACTORY_STOP_SERVICES:-sau-stack.service}"
ZSTD_LEVEL="${OMNIBULL_FACTORY_ZSTD_LEVEL:-19}"

ARCHIVES_DIR="${STAGING_DIR}/archives"
META_DIR="${STAGING_DIR}/meta"
PROVISIONING_DIR="${STAGING_DIR}/provisioning"
CHECKSUM_FILE="${STAGING_DIR}/SHA256SUMS"
PARTITION_MAP_FILE="${META_DIR}/partition-map.tsv"
SOURCE_FSTAB_FILE="${META_DIR}/source-fstab.txt"
SOURCE_FINDMNT_FILE="${META_DIR}/source-findmnt.txt"
SOURCE_LSBLK_FILE="${META_DIR}/source-lsblk.txt"
SOURCE_LAYOUT_FILE="${META_DIR}/disk-layout.sfdisk"
CAPTURE_ENV_FILE="${META_DIR}/capture.env"

ROOT_EXCLUDES_FILE="${META_DIR}/root-excludes.txt"
PERSISTENT_EXCLUDES_FILE="${META_DIR}/persistent-excludes.txt"

ROOT_CAPTURE_SOURCE=""
ROOT_DEVICE=""
EFI_DEVICE=""
BOOT_DEVICE=""
PERSISTENT_DEVICE=""
SWAP_DEVICE=""
declare -a STOPPED_SERVICES=()


log() {
  printf '[%s] [factory-capture] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
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
  require_cmd findmnt
  require_cmd lsblk
  require_cmd blkid
  require_cmd sha256sum
  require_cmd sfdisk
}


trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s\n' "${value}"
}


detect_source_disk() {
  if [[ -n "${SOURCE_DISK}" ]]; then
    printf '%s\n' "${SOURCE_DISK}"
    return 0
  fi

  local root_source parent_name
  root_source="$(findmnt -no SOURCE /)"
  parent_name="$(lsblk -no PKNAME "${root_source}")"
  if [[ -z "${parent_name}" ]]; then
    log "failed to detect source disk from root source ${root_source}"
    exit 1
  fi
  printf '/dev/%s\n' "${parent_name}"
}


resolve_root_capture_source() {
  local root_source
  root_source="$(findmnt -no SOURCE /)"
  if findmnt -rn -S "${root_source}" -o TARGET | grep -qx '/sysroot'; then
    printf '/sysroot\n'
    return 0
  fi
  printf '/\n'
}


setup_staging() {
  rm -rf "${STAGING_DIR}"
  mkdir -p "${ARCHIVES_DIR}" "${META_DIR}" "${PROVISIONING_DIR}" "${OUTPUT_PARENT}"
  : > "${META_DIR}/empty-excludes.txt"
}


write_exclude_files() {
  cat > "${ROOT_EXCLUDES_FILE}" <<'EOF'
./boot
./persistent
./home
./root
./var
./dev
./proc
./sys
./run
./tmp
./mnt
./media
./lost+found
./swapfile
EOF

  cat > "${PERSISTENT_EXCLUDES_FILE}" <<'EOF'
./home/*/.cache
./home/*/.config/google-chrome/Default/Cache
./home/*/.config/google-chrome/ShaderCache
./home/*/.local/share/Trash
./home/*/.npm/_cacache
./root/.cache
./root/.config/google-chrome/Default/Cache
./root/.config/google-chrome/ShaderCache
./home/*/Desktop/omnibull-factory-bundle-*
./home/*/Desktop/OmniBull-Deepin25-*.iso
./var/cache/apt/archives
./var/cache/apt/archives/partial
./var/cache/*
./var/tmp
./var/log
./var/log/*
./ostree/deploy/*/var/cache
./ostree/deploy/*/var/cache/*
./ostree/deploy/*/var/tmp
./ostree/deploy/*/var/tmp/*
./ostree/deploy/*/var/log
./ostree/deploy/*/var/log/*
./ostree/deploy/*/var/log/journal
./ostree/deploy/*/var/log/journal/*
./lost+found
EOF
}


stop_services() {
  local raw item
  local state
  IFS=',' read -r -a raw <<< "${STOP_SERVICES_RAW}"
  for item in "${raw[@]}"; do
    item="$(trim "${item}")"
    if [[ -z "${item}" ]]; then
      continue
    fi
    state="$(systemctl is-active "${item}" 2>/dev/null || true)"
    if [[ -n "${state}" && "${state}" != "inactive" && "${state}" != "failed" ]]; then
      log "stopping service ${item}"
      systemctl stop "${item}"
      STOPPED_SERVICES+=("${item}")
    fi
  done
}


restart_services() {
  local item
  for item in "${STOPPED_SERVICES[@]:-}"; do
    log "starting service ${item}"
    systemctl start "${item}" || true
  done
}


cleanup() {
  restart_services
}


write_metadata_files() {
  lsblk -o NAME,PATH,SIZE,TYPE,FSTYPE,MOUNTPOINT,LABEL,UUID,PARTUUID "${SOURCE_DISK}" > "${SOURCE_LSBLK_FILE}"
  findmnt -R / > "${SOURCE_FINDMNT_FILE}"
  cat /etc/fstab > "${SOURCE_FSTAB_FILE}"
  sfdisk --dump "${SOURCE_DISK}" > "${SOURCE_LAYOUT_FILE}"
}


record_partition() {
  local role="$1"
  local mountpoint="$2"
  local device fstype label uuid partuuid partnum

  if [[ "${mountpoint}" == "[SWAP]" ]]; then
    device="$(swapon --show=NAME --noheadings | head -n 1 | xargs)"
    fstype="swap"
  else
    device="$(findmnt -no SOURCE "${mountpoint}")"
    fstype="$(findmnt -no FSTYPE "${mountpoint}")"
  fi

  if [[ -z "${device}" ]]; then
    return 0
  fi

  label="$(blkid -s LABEL -o value "${device}" 2>/dev/null || true)"
  uuid="$(blkid -s UUID -o value "${device}" 2>/dev/null || true)"
  partuuid="$(blkid -s PARTUUID -o value "${device}" 2>/dev/null || true)"
  partnum="$(lsblk -no PARTN "${device}" 2>/dev/null | xargs || true)"
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "${role}" \
    "${partnum}" \
    "${fstype}" \
    "${mountpoint}" \
    "${label}" \
    "${uuid}" \
    "${partuuid}" >> "${PARTITION_MAP_FILE}"
}


write_partition_map() {
  : > "${PARTITION_MAP_FILE}"
  printf 'role\tpartnum\tfstype\tmountpoint\tlabel\tuuid\tpartuuid\n' >> "${PARTITION_MAP_FILE}"

  record_partition efi /boot/efi
  record_partition boot /boot
  record_partition swap "[SWAP]"
  record_partition root /
  if findmnt /persistent >/dev/null 2>&1; then
    record_partition persistent /persistent
  fi

  EFI_DEVICE="$(findmnt -no SOURCE /boot/efi)"
  BOOT_DEVICE="$(findmnt -no SOURCE /boot)"
  ROOT_DEVICE="$(findmnt -no SOURCE /)"
  if findmnt /persistent >/dev/null 2>&1; then
    PERSISTENT_DEVICE="$(findmnt -no SOURCE /persistent)"
  fi
  SWAP_DEVICE="$(swapon --show=NAME --noheadings | head -n 1 | xargs || true)"
}


write_capture_env() {
  ROOT_CAPTURE_SOURCE="$(resolve_root_capture_source)"
  cat > "${CAPTURE_ENV_FILE}" <<EOF
SOURCE_DISK=${SOURCE_DISK}
ROOT_CAPTURE_SOURCE=${ROOT_CAPTURE_SOURCE}
WORK_USER=${WORK_USER}
APP_ROOT=${ROOT_DIR}
CAPTURED_AT=$(date --iso-8601=seconds)
EOF
}


create_archive() {
  local source_path="$1"
  local archive_path="$2"
  local exclude_file="$3"

  log "creating archive $(basename "${archive_path}") from ${source_path}"
  tar \
    --one-file-system \
    --numeric-owner \
    --acls \
    --xattrs \
    --exclude-from "${exclude_file}" \
    -C "${source_path}" \
    -cpf - . \
    | zstd -T0 -"${ZSTD_LEVEL}" -o "${archive_path}"
}


copy_bundle_helpers() {
  cp "${ROOT_DIR}/scripts/factory_restore_bundle.sh" "${STAGING_DIR}/restore.sh"
  chmod +x "${STAGING_DIR}/restore.sh"
  cp "${ROOT_DIR}/docs/omnibull_factory_fast_mode.md" "${STAGING_DIR}/README_FACTORY.md"

  cat > "${PROVISIONING_DIR}/README.txt" <<'EOF'
Optional supplier/device overrides live here.

Supported files:

1. rootfs/
   Any file tree placed under provisioning/rootfs/ will be copied into the
   restored target filesystem after the base archives are extracted.

2. post-restore.sh
   If present, restore.sh will copy it into the target rootfs and execute it
   inside chroot before unmounting the target disk.

3. remove_device_identity
   restore.sh removes OmniBull device identity files by default so each cloned
   machine auto-generates a fresh deviceCode/agentKey/localApiKey on first boot.
EOF
}


write_checksums() {
  (
    cd "${STAGING_DIR}"
    find archives meta -type f -print0 \
      | sort -z \
      | xargs -0 sha256sum
  ) > "${CHECKSUM_FILE}"
}


finalize_bundle() {
  rm -rf "${FINAL_DIR}"
  mv "${STAGING_DIR}" "${FINAL_DIR}"
  chown -R "${WORK_USER}:${WORK_USER}" "${FINAL_DIR}" || true
}


main() {
  ensure_requirements
  SOURCE_DISK="$(detect_source_disk)"
  setup_staging
  write_exclude_files
  trap cleanup EXIT
  stop_services
  write_metadata_files
  write_partition_map
  write_capture_env

  create_archive /boot/efi "${ARCHIVES_DIR}/efi.tar.zst" "${META_DIR}/empty-excludes.txt"
  create_archive /boot "${ARCHIVES_DIR}/boot.tar.zst" "${META_DIR}/empty-excludes.txt"
  create_archive "${ROOT_CAPTURE_SOURCE}" "${ARCHIVES_DIR}/root.tar.zst" "${ROOT_EXCLUDES_FILE}"

  if [[ -n "${PERSISTENT_DEVICE}" ]]; then
    create_archive /persistent "${ARCHIVES_DIR}/persistent.tar.zst" "${PERSISTENT_EXCLUDES_FILE}"
  fi

  copy_bundle_helpers
  write_checksums
  finalize_bundle
  trap - EXIT
  cleanup

  log "factory bundle created at ${FINAL_DIR}"
}

main "$@"
