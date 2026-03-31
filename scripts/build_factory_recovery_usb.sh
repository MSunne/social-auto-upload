#!/usr/bin/env bash

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

usage() {
  cat <<'EOF'
Usage:
  build_factory_recovery_usb.sh <source-iso-or-device> <target-usb-disk> <factory-bundle-dir>

Examples:
  sudo bash scripts/build_factory_recovery_usb.sh /dev/sdb /dev/sdb /home/sun/Desktop/omnibull-factory-bundle-20260330-163103
  sudo bash scripts/build_factory_recovery_usb.sh /path/to/deepin.iso /dev/sdb /path/to/omnibull-factory-bundle

Environment overrides:
  OMNIBULL_RECOVERY_OUTPUT_DIR   Directory for the generated recovery ISO
  OMNIBULL_RECOVERY_OUTPUT_ISO   Explicit path for the generated recovery ISO
  OMNIBULL_RECOVERY_LABEL        Label of the bundle partition on the USB (default: OMNIFACTORY)
  OMNIBULL_RECOVERY_WORK_DIR     Working directory used during remastering
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

build_input_spec() {
  local source="$1"
  if [[ -b "${source}" ]]; then
    printf 'stdio:%s\n' "${source}"
    return 0
  fi
  printf '%s\n' "${source}"
}

extract_iso_path() {
  local iso_spec="$1"
  local iso_path="$2"
  local destination="$3"
  mkdir -p "$(dirname "${destination}")"
  xorriso -osirrox on -indev "${iso_spec}" -extract "${iso_path}" "${destination}" >/dev/null
}

append_module_if_missing() {
  local module_file="$1"
  local module_name="$2"
  if grep -qxF "${module_name}" "${module_file}"; then
    return 0
  fi
  printf '%s\n' "${module_name}" >> "${module_file}"
}

render_grub_cfg() {
  local output="$1"
  cat > "${output}" <<EOF
if loadfont /boot/grub/unicode.pf2 ; then
	set gfxmode=1920x1080,1680x1050,1600x900,1280x1024,1280x800,1280x720,1024x768,auto
	insmod efi_gop
	insmod efi_uga
	insmod gfxterm
	terminal_output gfxterm
fi

set menu_color_normal=white/black
set menu_color_highlight=black/light-gray
set theme=/boot/grub/themes/deepin/theme.txt
set default=0
set timeout=4

menuentry "OmniBull Factory Restore" {
	set gfxpayload=keep
	linux /live/vmlinuz.efi boot=live union=overlay locales=zh_CN.UTF-8 console=tty systemd.unit=multi-user.target omni.factory.recovery=1 quiet splash --
	initrd /live/initrd
}

menuentry "Try Deepin Desktop 25" {
	set gfxpayload=keep
	linux /live/vmlinuz.efi boot=live union=overlay locales=zh_CN.UTF-8 console=tty splash --
	initrd /live/initrd
}

menuentry "Install Deepin 25 with kernel 6.6 desktop" {
	set gfxpayload=keep
	linux /live/vmlinuz-6.6 boot=live union=overlay livecd-installer locales=zh_CN.UTF-8 console=tty splash --
	initrd /live/initrd-6.6
}

menuentry "Install Deepin 25 with kernel 6.6 desktop (Installation without graphics driver)" {
	set gfxpayload=keep
	linux /live/vmlinuz-6.6 boot=live union=overlay livecd-installer locales=zh_CN.UTF-8 console=tty splash nomodeset --
	initrd /live/initrd-6.6
}

menuentry "Install Deepin 25 with kernel 6.12 desktop" {
	set gfxpayload=keep
	linux /live/vmlinuz.efi boot=live union=overlay livecd-installer locales=zh_CN.UTF-8 console=tty splash --
	initrd /live/initrd
}

menuentry "Install Deepin 25 with kernel 6.12 desktop (Installation without graphics driver)" {
	set gfxpayload=keep
	linux /live/vmlinuz.efi boot=live union=overlay livecd-installer locales=zh_CN.UTF-8 console=tty splash nomodeset --
	initrd /live/initrd
}
EOF
}

render_isolinux_cfg() {
  local output="$1"
  cat > "${output}" <<EOF
label omnibull-restore
	menu label ^OmniBull Factory Restore
	menu default
	linux /live/vmlinuz.efi
	initrd /live/initrd
	append boot=live components quiet splash union=overlay locales=zh_CN.UTF-8 systemd.unit=multi-user.target omni.factory.recovery=1

label deepin-live
	menu label ^Try Deepin 25 desktop
	linux /live/vmlinuz.efi
	initrd /live/initrd
	append boot=live components quiet splash union=overlay locales=zh_CN.UTF-8

label deepin-install-66
	menu label ^Install Deepin 25 with kernel 6.6 desktop
	linux /live/vmlinuz-6.6
	initrd /live/initrd-6.6
	append boot=live components quiet splash union=overlay livecd-installer locales=zh_CN.UTF-8

label deepin-install-66-safe
	menu label ^Install Deepin 25 with kernel 6.6 desktop (Safe graphics,Please use this mode to install if your graphics card is not working properly.)
	linux /live/vmlinuz-6.6
	initrd /live/initrd-6.6
	append boot=live components quiet splash nomodeset union=overlay livecd-installer locales=zh_CN.UTF-8

label deepin-install-612
	menu label ^Install Deepin 25 with kernel 6.12 desktop
	linux /live/vmlinuz
	initrd /live/initrd
	append boot=live components quiet splash union=overlay livecd-installer locales=zh_CN.UTF-8

label deepin-install-612-safe
	menu label ^Install Deepin 25 with kernel 6.12 desktop (Safe graphics,Please use this mode to install if your graphics card is not working properly.)
	linux /live/vmlinuz
	initrd /live/initrd
	append boot=live components quiet splash nomodeset union=overlay livecd-installer locales=zh_CN.UTF-8
EOF
}

build_overlay_tree() {
  local overlay_root="$1"
  local bundle_label="$2"

  mkdir -p \
    "${overlay_root}/usr/local/sbin" \
    "${overlay_root}/etc/systemd/system/multi-user.target.wants"

  cat > "${overlay_root}/usr/local/sbin/omnibull-recovery-runner" <<EOF
#!/usr/bin/env bash

set -euo pipefail

export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

BUNDLE_LABEL="${bundle_label}"
MOUNT_ROOT="/mnt/omnibull-factory"

exec </dev/tty1 >/dev/tty1 2>&1
clear
setterm -blank 0 -powersave off -powerdown 0 2>/dev/null || true

say() {
  printf '[%s] %s\n' "\$(date '+%H:%M:%S')" "\$*"
}

pause_shell() {
  echo
  read -r -p "Press Enter to open a rescue shell..." _
  exec /bin/bash -l
}

find_bundle_device() {
  local dev
  dev="\$(blkid -o device -t LABEL="\${BUNDLE_LABEL}" | head -n 1 || true)"
  if [[ -n "\${dev}" ]]; then
    printf '%s\n' "\${dev}"
    return 0
  fi
  return 1
}

mount_bundle() {
  local dev="\$1"
  mkdir -p "\${MOUNT_ROOT}"
  if ! mountpoint -q "\${MOUNT_ROOT}"; then
    mount -o ro "\${dev}" "\${MOUNT_ROOT}"
  fi
}

find_bundle_dir() {
  local restore_path
  restore_path="\$(find "\${MOUNT_ROOT}" -maxdepth 3 -type f -name restore.sh | sort | head -n 1 || true)"
  if [[ -z "\${restore_path}" ]]; then
    return 1
  fi
  dirname "\${restore_path}"
}

collect_targets() {
  local bundle_dev="\$1"
  local bundle_parent=""
  local line name type rm tran size model

  bundle_parent="\$(lsblk -no PKNAME "\${bundle_dev}" 2>/dev/null || true)"
  while read -r name type rm tran size model; do
    [[ "\${type}" == "disk" ]] || continue
    [[ -n "\${name}" ]] || continue
    if [[ -n "\${bundle_parent}" && "/dev/\${name}" == "/dev/\${bundle_parent}" ]]; then
      continue
    fi
    if [[ "\${rm}" == "1" && "\${tran}" == "usb" ]]; then
      continue
    fi
    printf '/dev/%s\t%s\t%s\t%s\n' "\${name}" "\${size}" "\${tran}" "\${model}"
  done < <(lsblk -dn -o NAME,TYPE,RM,TRAN,SIZE,MODEL)
}

main() {
  echo "OmniBull Factory Recovery"
  echo
  say "Locating bundle partition labeled \${BUNDLE_LABEL}"

  local bundle_dev
  if ! bundle_dev="\$(find_bundle_device)"; then
    say "Bundle partition not found."
    pause_shell
  fi

  say "Mounting \${bundle_dev}"
  mount_bundle "\${bundle_dev}"

  local bundle_dir
  if ! bundle_dir="\$(find_bundle_dir)"; then
    say "restore.sh was not found on \${bundle_dev}"
    pause_shell
  fi

  local targets=()
  while IFS= read -r row; do
    [[ -n "\${row}" ]] || continue
    targets+=("\${row}")
  done < <(collect_targets "\${bundle_dev}")

  if [[ "\${#targets[@]}" -eq 0 ]]; then
    say "No target disks were detected."
    pause_shell
  fi

  echo
  echo "Detected candidate target disks:"
  local row path size tran model
  for row in "\${targets[@]}"; do
    IFS=$'\t' read -r path size tran model <<<"\${row}"
    printf '  %s  size=%s  bus=%s  model=%s\n' "\${path}" "\${size}" "\${tran:-unknown}" "\${model:-unknown}"
  done

  local default_target=""
  if [[ "\${#targets[@]}" -eq 1 ]]; then
    IFS=$'\t' read -r default_target _ <<<"\${targets[0]}"
  fi

  echo
  read -r -p "Target disk [\${default_target:-/dev/sda}]: " target_disk
  target_disk="\${target_disk:-\${default_target:-/dev/sda}}"

  if [[ ! -b "\${target_disk}" ]]; then
    say "Target disk does not exist: \${target_disk}"
    pause_shell
  fi

  if [[ "\${target_disk}" == "\${bundle_dev}" || "\${target_disk}" == "/dev/\$(lsblk -no PKNAME "\${bundle_dev}")" ]]; then
    say "Target disk cannot be the recovery USB itself."
    pause_shell
  fi

  echo
  lsblk -o PATH,SIZE,FSTYPE,LABEL,MODEL "\${target_disk}" || true
  echo
  echo "This will erase all partitions on \${target_disk}."
  read -r -p "Type RESTORE to continue: " confirm
  if [[ "\${confirm}" != "RESTORE" ]]; then
    say "Confirmation did not match. Aborting."
    pause_shell
  fi

  export OMNIBULL_FACTORY_BUNDLE_DIR="\${bundle_dir}"
  say "Starting restore using bundle \${bundle_dir}"
  bash "\${bundle_dir}/restore.sh" "\${target_disk}"

  say "Restore completed."
  echo
  echo "Remove the recovery USB if needed, then press Enter to reboot."
  read -r _
  sync
  reboot -f
}

main "\$@"
EOF
  chmod 0755 "${overlay_root}/usr/local/sbin/omnibull-recovery-runner"

  cat > "${overlay_root}/etc/systemd/system/omnibull-recovery.service" <<'EOF'
[Unit]
Description=OmniBull factory restore runner
ConditionKernelCommandLine=omni.factory.recovery=1
After=local-fs.target systemd-user-sessions.service
Before=getty@tty1.service display-manager.service
Conflicts=getty@tty1.service display-manager.service

[Service]
Type=simple
ExecStart=/usr/local/sbin/omnibull-recovery-runner
StandardInput=tty-force
StandardOutput=tty
StandardError=tty
TTYPath=/dev/tty1
TTYReset=yes
TTYVHangup=yes
TTYVTDisallocate=yes
Restart=no

[Install]
WantedBy=multi-user.target
EOF

  ln -sf ../omnibull-recovery.service \
    "${overlay_root}/etc/systemd/system/multi-user.target.wants/omnibull-recovery.service"
}

build_recovery_iso() {
  local source_spec="$1"
  local build_root="$2"
  local output_iso="$3"
  local bundle_label="$4"

  local grub_cfg="${build_root}/boot-grub-grub.cfg"
  local isolinux_cfg="${build_root}/isolinux-live.cfg"
  local module_file="${build_root}/filesystem.module"
  local overlay_root="${build_root}/overlay"
  local overlay_sqfs="${build_root}/filesystem-omnibull.squashfs"

  extract_iso_path "${source_spec}" /live/filesystem.module "${module_file}"
  render_grub_cfg "${grub_cfg}"
  render_isolinux_cfg "${isolinux_cfg}"
  build_overlay_tree "${overlay_root}" "${bundle_label}"
  append_module_if_missing "${module_file}" filesystem-omnibull.squashfs
  mksquashfs "${overlay_root}" "${overlay_sqfs}" -noappend -comp xz >/dev/null

  rm -f "${output_iso}"
  set +e
  xorriso \
    -indev "${source_spec}" \
    -outdev "${output_iso}" \
    -boot_image any replay \
    -overwrite on \
    -map "${grub_cfg}" /boot/grub/grub.cfg \
    -map "${isolinux_cfg}" /isolinux/live.cfg \
    -map "${module_file}" /live/filesystem.module \
    -map "${overlay_sqfs}" /live/filesystem-omnibull.squashfs \
    -commit \
    -end >/dev/null
  local xorriso_status=$?
  set -e
  if [[ "${xorriso_status}" -ne 0 && "${xorriso_status}" -ne 32 ]]; then
    echo "xorriso failed with exit code ${xorriso_status}" >&2
    exit "${xorriso_status}"
  fi
}

find_last_free_start_mib() {
  local disk="$1"
  parted -sm "${disk}" unit MiB print free \
    | awk -F: '$1 == "" && $5 == "free;" { gsub("MiB", "", $2); start = $2 } END { if (start == "") exit 1; print start }'
}

write_bundle_partition() {
  local disk="$1"
  local bundle_dir="$2"
  local bundle_label="$3"
  local mount_root="$4"

  local free_start
  free_start="$(find_last_free_start_mib "${disk}")"
  parted -s "${disk}" mkpart "${bundle_label}" ext4 "${free_start}MiB" 100%
  partprobe "${disk}" || true
  udevadm settle || true

  local data_part
  data_part="$(part_path "${disk}" 3)"
  mkfs.ext4 -F -L "${bundle_label}" "${data_part}" >/dev/null

  mkdir -p "${mount_root}"
  mount "${data_part}" "${mount_root}"
  mkdir -p "${mount_root}/$(basename "${bundle_dir}")"
  rsync -aH --info=progress2 "${bundle_dir}/" "${mount_root}/$(basename "${bundle_dir}")/"
  cat > "${mount_root}/START_HERE.txt" <<EOF
OmniBull Factory Recovery USB

This USB boots directly into OmniBull Factory Restore.
If auto-start does not happen, choose "OmniBull Factory Restore" from the boot menu.

Bundle location:
  $(basename "${bundle_dir}")

Manual recovery command inside a Linux rescue environment:
  sudo bash /media/${bundle_label}/$(basename "${bundle_dir}")/restore.sh /dev/sda
EOF
  sync
  umount "${mount_root}"
}

main() {
  if [[ $# -ne 3 ]]; then
    usage >&2
    exit 1
  fi

  local source_image="$1"
  local target_usb="$2"
  local bundle_dir="$3"
  local bundle_label="${OMNIBULL_RECOVERY_LABEL:-OMNIFACTORY}"
  local timestamp
  timestamp="$(date +%Y%m%d-%H%M%S)"
  local user_home="${SUDO_USER:+/home/${SUDO_USER}}"
  if [[ -z "${user_home}" || ! -d "${user_home}" ]]; then
    user_home="/root"
  fi
  local output_dir="${OMNIBULL_RECOVERY_OUTPUT_DIR:-${user_home}/Desktop}"
  local output_iso="${OMNIBULL_RECOVERY_OUTPUT_ISO:-${output_dir}/OmniBull-Recovery-${timestamp}.iso}"
  local build_root="${OMNIBULL_RECOVERY_WORK_DIR:-/var/tmp/omnibull-recovery-usb-${timestamp}}"
  local data_mount="${build_root}/bundle-mount"
  local source_spec
  source_spec="$(build_input_spec "${source_image}")"

  if [[ ! -e "${source_image}" ]]; then
    echo "source image or device not found: ${source_image}" >&2
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

  require_cmd xorriso
  require_cmd mksquashfs
  require_cmd parted
  require_cmd partprobe
  require_cmd udevadm
  require_cmd mkfs.ext4
  require_cmd rsync
  require_cmd dd
  require_cmd lsblk

  mkdir -p "${output_dir}"
  rm -rf "${build_root}"
  mkdir -p "${build_root}"
  trap 'umount -R "'"${data_mount}"'" 2>/dev/null || true; rm -rf "'"${build_root}"'"' EXIT

  echo "Building recovery ISO at ${output_iso}"
  build_recovery_iso "${source_spec}" "${build_root}" "${output_iso}" "${bundle_label}"

  echo "Writing recovery ISO to ${target_usb}"
  unmount_disk_partitions "${target_usb}"
  dd if="${output_iso}" of="${target_usb}" bs=16M oflag=direct status=progress conv=fsync
  sync
  partprobe "${target_usb}" || true
  udevadm settle || true

  echo "Creating bundle partition on ${target_usb}"
  write_bundle_partition "${target_usb}" "${bundle_dir}" "${bundle_label}" "${data_mount}"

  if [[ -n "${SUDO_USER:-}" ]]; then
    chown "${SUDO_USER}:${SUDO_USER}" "${output_iso}"
  fi

  echo "Recovery USB is ready: ${target_usb}"
  echo "Recovery ISO saved at: ${output_iso}"
}

main "$@"
