#!/bin/bash
set -euo pipefail

WORKDIR="${WORKDIR:-$HOME/factory-usb-work}"
USB_DISK="${USB_DISK:-/dev/sdb}"
EFI_PART="${USB_DISK}1"
ROOT_PART="${USB_DISK}2"
MNT="${MNT:-/mnt/factory-usb-root}"

mkdir -p "$MNT"
if ! mountpoint -q "$MNT"; then
  mount "$ROOT_PART" "$MNT"
fi

mkdir -p "$MNT/boot/efi"
if ! mountpoint -q "$MNT/boot/efi"; then
  mount "$EFI_PART" "$MNT/boot/efi"
fi

cp /etc/resolv.conf "$MNT/etc/resolv.conf"

if ! mountpoint -q "$MNT/dev"; then
  mount --bind /dev "$MNT/dev"
fi
if ! mountpoint -q "$MNT/dev/pts"; then
  mount --bind /dev/pts "$MNT/dev/pts"
fi
if ! mountpoint -q "$MNT/proc"; then
  mount --bind /proc "$MNT/proc"
fi
if ! mountpoint -q "$MNT/sys"; then
  mount --bind /sys "$MNT/sys"
fi
if ! mountpoint -q "$MNT/run"; then
  mount --bind /run "$MNT/run"
fi

cp "$WORKDIR/factory-runner.sh" "$MNT/tmp/factory-runner.sh"
cp "$WORKDIR/factory-usb.service" "$MNT/tmp/factory-usb.service"
cp "$WORKDIR/README-supplier.txt" "$MNT/tmp/README-supplier.txt"

cat >"$MNT/tmp/finalize-chroot.sh" <<'EOF_CHROOT'
#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update
mkdir -p /run/apt-archives/partial
apt-get -o Dir::Cache::Archives=/run/apt-archives install -y initramfs-tools

mkdir -p /home/partimag /var/log/factory-usb /usr/local/sbin
cp /tmp/factory-runner.sh /usr/local/sbin/factory-runner.sh
cp /tmp/factory-usb.service /etc/systemd/system/factory-usb.service
cp /tmp/README-supplier.txt /root/README-supplier.txt
chmod 0755 /usr/local/sbin/factory-runner.sh
ln -sf ../factory-usb.service /etc/systemd/system/multi-user.target.wants/factory-usb.service

grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=FACTORYUSB --removable --no-nvram
update-initramfs -c -k all
update-grub
EOF_CHROOT

chmod 0755 "$MNT/tmp/finalize-chroot.sh"
chroot "$MNT" /bin/bash /tmp/finalize-chroot.sh

uuid_root="$(blkid -s UUID -o value "$ROOT_PART")"
uuid_efi="$(blkid -s UUID -o value "$EFI_PART")"
cat >"$MNT/etc/fstab" <<EOF_FSTAB
UUID=$uuid_root / ext4 defaults 0 1
UUID=$uuid_efi /boot/efi vfat umask=0077 0 1
tmpfs /tmp tmpfs defaults,nosuid 0 0
EOF_FSTAB

rm -f \
  "$MNT/tmp/finalize-chroot.sh" \
  "$MNT/tmp/inside-chroot.sh" \
  "$MNT/tmp/factory-runner.sh" \
  "$MNT/tmp/factory-usb.service" \
  "$MNT/tmp/README-supplier.txt"
sync

umount "$MNT/run"
umount "$MNT/sys"
umount "$MNT/proc"
umount "$MNT/dev/pts"
umount "$MNT/dev"
umount "$MNT/boot/efi"
umount "$MNT"

lsblk -o NAME,PATH,SIZE,RM,RO,TYPE,FSTYPE,MOUNTPOINT,LABEL "$USB_DISK"
