#!/bin/bash
set -euo pipefail

IMG_NAME="golden-master"
FIRSTBOOT_SERVICE_NAME="factory-firstboot.service"
FIRSTBOOT_SCRIPT_PATH="/usr/local/sbin/factory-firstboot.sh"

USB_ROOT_PART=""
USB_BOOT_DISK=""
USB_REPO_DIR="/home/partimag"
TARGET_DISK=""
LOG_DIR="/var/log/factory-usb"
LOG_FILE="$LOG_DIR/factory-runner-$(date '+%Y%m%d-%H%M%S').log"

mkdir -p "$USB_REPO_DIR" "$LOG_DIR"

log() {
  local now
  now="$(date '+%F %T')"
  echo "[$now] $*" | tee -a "$LOG_FILE"
}

die() {
  log "ERROR: $*"
  echo
  echo "Press Enter to continue..."
  read -r _
  exit 1
}

find_usb_root() {
  USB_ROOT_PART="$(findmnt -n -o SOURCE / 2>/dev/null || true)"
  [ -b "$USB_ROOT_PART" ] || die "Unable to identify the USB root partition."

  USB_BOOT_DISK="/dev/$(lsblk -no PKNAME "$USB_ROOT_PART" 2>/dev/null || true)"
  [ -b "$USB_BOOT_DISK" ] || die "Unable to identify the USB boot disk."
}

find_target_disk() {
  TARGET_DISK="$(
    lsblk -dnpo NAME,TYPE,RM,TRAN |
      awk -v boot="$USB_BOOT_DISK" '$2 == "disk" && $1 != boot && $3 == "0" { print $1; exit }'
  )"
  [ -b "$TARGET_DISK" ] || die "Unable to find the internal target disk."
}

target_disk_has_partitions() {
  [ "$(lsblk -lnpo NAME "$TARGET_DISK" | wc -l)" -gt 1 ]
}

image_exists() {
  [ -d "$USB_REPO_DIR/$IMG_NAME" ]
}

part_suffix() {
  local disk="$1"
  case "$disk" in
    *[0-9]) printf 'p' ;;
    *) printf '' ;;
  esac
}

find_partition_by_label() {
  local disk="$1"
  local label="$2"
  lsblk -lnpo NAME,LABEL "$disk" | awk -v want="$label" '$2 == want { print $1; exit }'
}

countdown() {
  local seconds="$1"
  local message="$2"
  local remaining

  echo "$message"
  for ((remaining = seconds; remaining >= 1; remaining--)); do
    printf '\rStarting in %2d second(s)... Press Ctrl+C to cancel. ' "$remaining"
    sleep 1
  done
  printf '\n'
}

capture_master() {
  local disk_name backup_name
  disk_name="$(basename "$TARGET_DISK")"
  backup_name="${IMG_NAME}.bak-$(date '+%Y%m%d-%H%M%S')"

  if image_exists; then
    log "Existing image found. Renaming $IMG_NAME to $backup_name."
    mv "$USB_REPO_DIR/$IMG_NAME" "$USB_REPO_DIR/$backup_name"
  fi

  log "Saving disk $TARGET_DISK to image $IMG_NAME under $USB_REPO_DIR."
  ocs-sr -q2 -j2 -z1p -i 2000 -scs -p true savedisk "$IMG_NAME" "$disk_name"
  sync

  log "Capture completed successfully."
  echo
  echo "Golden image has been saved to this USB drive."
  echo "You can now use the same USB drive to restore blank machines."
  echo
  echo "Press Enter to power off..."
  read -r _
  poweroff
}

inject_firstboot() {
  local root_part persistent_part target_suffix target_mount

  target_suffix="$(part_suffix "$TARGET_DISK")"
  root_part="$(find_partition_by_label "$TARGET_DISK" "Roota")"
  persistent_part="$(find_partition_by_label "$TARGET_DISK" "_dde_data")"

  [ -n "$root_part" ] || root_part="${TARGET_DISK}${target_suffix}4"
  [ -n "$persistent_part" ] || persistent_part="${TARGET_DISK}${target_suffix}5"

  [ -b "$root_part" ] || die "Cannot find restored root partition."
  [ -b "$persistent_part" ] || die "Cannot find restored persistent partition."

  target_mount="$(mktemp -d /tmp/factory-root.XXXXXX)"
  trap 'umount "$target_mount/persistent" >/dev/null 2>&1 || true; umount "$target_mount" >/dev/null 2>&1 || true; rm -rf "$target_mount"' RETURN

  mount "$root_part" "$target_mount"
  mkdir -p "$target_mount/persistent"
  mount "$persistent_part" "$target_mount/persistent"

  mkdir -p "$target_mount/usr/local/sbin"
  mkdir -p "$target_mount/etc/systemd/system/multi-user.target.wants"

  cat >"$target_mount/$FIRSTBOOT_SCRIPT_PATH" <<'EOF_SCRIPT'
#!/bin/bash
set -euo pipefail
exec >/var/log/factory-firstboot.log 2>&1

echo "[factory-firstboot] Regenerating machine identity..."
rm -f /etc/machine-id /var/lib/dbus/machine-id
systemd-machine-id-setup

echo "[factory-firstboot] Regenerating SSH host keys..."
rm -f /etc/ssh/ssh_host_*
ssh-keygen -A

touch /var/lib/factory-firstboot.done

systemctl disable factory-firstboot.service || true
rm -f /etc/systemd/system/multi-user.target.wants/factory-firstboot.service
rm -f /etc/systemd/system/factory-firstboot.service
rm -f /usr/local/sbin/factory-firstboot.sh
exit 0
EOF_SCRIPT

  chmod 0755 "$target_mount/$FIRSTBOOT_SCRIPT_PATH"

  cat >"$target_mount/etc/systemd/system/$FIRSTBOOT_SERVICE_NAME" <<'EOF_SERVICE'
[Unit]
Description=Factory first boot unique identity setup
After=local-fs.target
Before=ssh.service sshd.service multi-user.target
ConditionPathExists=!/var/lib/factory-firstboot.done

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/factory-firstboot.sh
RemainAfterExit=no

[Install]
WantedBy=multi-user.target
EOF_SERVICE

  ln -sf "../$FIRSTBOOT_SERVICE_NAME" \
    "$target_mount/etc/systemd/system/multi-user.target.wants/$FIRSTBOOT_SERVICE_NAME"

  sync
  umount "$target_mount/persistent"
  umount "$target_mount"
  rm -rf "$target_mount"
  trap - RETURN
}

restore_target() {
  local disk_name answer
  disk_name="$(basename "$TARGET_DISK")"

  image_exists || die "No golden image was found on this USB drive."

  if target_disk_has_partitions; then
    echo
    echo "Target disk $TARGET_DISK already contains partitions."
    echo "Restore will ERASE the entire disk."
    printf 'Type YES to continue: '
    read -r answer
    [ "$answer" = "YES" ] || die "Restore cancelled."
  else
    countdown 15 "Blank internal SSD detected. The restore will start automatically."
  fi

  log "Restoring image $IMG_NAME to $TARGET_DISK."
  ocs-sr -g auto -e1 auto -e2 -r -j2 -k1 -p true restoredisk "$IMG_NAME" "$disk_name"
  sync

  log "Injecting first boot uniqueness service into restored system."
  inject_firstboot
  sync

  log "Restore completed successfully."
  echo
  echo "Restore completed. Remove the USB drive now."
  echo
  echo "Press Enter to power off..."
  read -r _
  poweroff
}

show_main_menu() {
  local answer=""

  clear || true
  echo "==============================================="
  echo " Factory USB for OmniBull / Deepin deployment"
  echo "==============================================="
  echo "USB boot disk : $USB_BOOT_DISK"
  echo "Target disk   : $TARGET_DISK"
  echo "Image path    : $USB_REPO_DIR/$IMG_NAME"
  echo

  if image_exists && ! target_disk_has_partitions; then
    echo "Mode:"
    echo "  1) Capture this machine as the new golden image"
    echo "  2) Restore golden image to the blank internal SSD"
    echo
    printf 'Select [2] in 15 seconds: '
    read -r -t 15 answer || true
    answer="${answer:-2}"
  elif image_exists; then
    echo "Mode:"
    echo "  1) Capture this machine as the new golden image"
    echo "  2) Restore golden image to the internal SSD"
    echo
    printf 'Select [1/2]: '
    read -r answer
  elif target_disk_has_partitions; then
    echo "No golden image exists on this USB drive yet."
    echo "This boot session will capture the current machine as the golden image."
    echo
    printf 'Press Enter to continue, or type q to quit: '
    read -r answer
    answer="${answer:-1}"
  else
    die "No golden image exists on this USB drive, and the internal disk is blank. Boot the approved master machine from this USB first, then capture the golden image."
  fi

  case "$answer" in
    1|"")
      capture_master
      ;;
    2)
      restore_target
      ;;
    q|Q)
      exit 0
      ;;
    *)
      die "Unknown selection: $answer"
      ;;
  esac
}

main() {
  export ocsroot="$USB_REPO_DIR"
  find_usb_root
  find_target_disk
  log "Booted from $USB_BOOT_DISK mounted on root filesystem."
  log "Selected target disk: $TARGET_DISK."
  show_main_menu
}

main "$@"
