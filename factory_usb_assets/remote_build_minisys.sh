#!/bin/bash
set -euo pipefail

WORKDIR="${WORKDIR:-$HOME/factory-usb-work}"
USB_DISK="${USB_DISK:-/dev/sdb}"
EFI_PART="${USB_DISK}1"
ROOT_PART="${USB_DISK}2"
MNT="${MNT:-/mnt/factory-usb-root}"

cleanup() {
  set +e
  sync
  mountpoint -q "$MNT/run" && umount "$MNT/run"
  mountpoint -q "$MNT/sys" && umount "$MNT/sys"
  mountpoint -q "$MNT/proc" && umount "$MNT/proc"
  mountpoint -q "$MNT/dev/pts" && umount "$MNT/dev/pts"
  mountpoint -q "$MNT/dev" && umount "$MNT/dev"
  mountpoint -q "$MNT/boot/efi" && umount "$MNT/boot/efi"
  mountpoint -q "$MNT" && umount "$MNT"
}

trap cleanup EXIT

umount "${USB_DISK}"* 2>/dev/null || true
wipefs -a "$USB_DISK"
parted -s "$USB_DISK" mklabel gpt
parted -s "$USB_DISK" mkpart ESP fat32 1MiB 513MiB
parted -s "$USB_DISK" set 1 esp on
parted -s "$USB_DISK" mkpart FACTORYROOT ext4 513MiB 100%
udevadm settle

mkfs.vfat -F 32 -n FACTORYEFI "$EFI_PART"
mkfs.ext2 -F -L FACTORYROOT -I 256 -m 0 "$ROOT_PART"
udevadm settle
sleep 2
umount "$EFI_PART" 2>/dev/null || true
umount "$ROOT_PART" 2>/dev/null || true

mkdir -p "$MNT"
mount "$ROOT_PART" "$MNT"
mkdir -p "$MNT/boot/efi"
mount "$EFI_PART" "$MNT/boot/efi"

debootstrap --arch amd64 crimson "$MNT" https://community-packages.deepin.com/beige

cat >"$MNT/etc/apt/sources.list" <<'EOF_SOURCES'
deb https://community-packages.deepin.com/beige/ crimson main commercial community
EOF_SOURCES

mkdir -p "$MNT/etc/apt/trusted.gpg.d" "$MNT/usr/share/keyrings"
cp -a /etc/apt/trusted.gpg.d/. "$MNT/etc/apt/trusted.gpg.d/" 2>/dev/null || true
cp -a /usr/share/keyrings/. "$MNT/usr/share/keyrings/" 2>/dev/null || true

cp /etc/resolv.conf "$MNT/etc/resolv.conf"
mount --bind /dev "$MNT/dev"
mount --bind /dev/pts "$MNT/dev/pts"
mount --bind /proc "$MNT/proc"
mount --bind /sys "$MNT/sys"
mount --bind /run "$MNT/run"

cat >"$MNT/tmp/inside-chroot.sh" <<'EOF_CHROOT'
#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

apt-get update
mkdir -p /run/apt-archives/partial
apt-get -o Dir::Cache::Archives=/run/apt-archives install -y linux-image-deepin-amd64 initramfs-tools systemd-sysv grub-efi-amd64 grub-efi-amd64-bin grub2-common clonezilla gdisk dosfstools parted e2fsprogs dialog sudo ca-certificates wget curl

echo factory-usb > /etc/hostname
cat >/etc/hosts <<'EOF_HOSTS'
127.0.0.1 localhost
127.0.1.1 factory-usb
EOF_HOSTS

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

cp "$WORKDIR/factory-runner.sh" "$MNT/tmp/factory-runner.sh"
cp "$WORKDIR/factory-usb.service" "$MNT/tmp/factory-usb.service"
cp "$WORKDIR/README-supplier.txt" "$MNT/tmp/README-supplier.txt"

chroot "$MNT" /bin/bash /tmp/inside-chroot.sh

uuid_root="$(blkid -s UUID -o value "$ROOT_PART")"
uuid_efi="$(blkid -s UUID -o value "$EFI_PART")"
cat >"$MNT/etc/fstab" <<EOF_FSTAB
UUID=$uuid_root / ext2 defaults 0 1
UUID=$uuid_efi /boot/efi vfat umask=0077 0 1
tmpfs /tmp tmpfs defaults,nosuid 0 0
EOF_FSTAB

rm -f "$MNT/tmp/inside-chroot.sh" "$MNT/tmp/factory-runner.sh" "$MNT/tmp/factory-usb.service" "$MNT/tmp/README-supplier.txt"
sync

trap - EXIT
cleanup

lsblk -o NAME,PATH,SIZE,RM,RO,TYPE,FSTYPE,MOUNTPOINT,LABEL "$USB_DISK"
