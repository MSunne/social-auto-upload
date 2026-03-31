import ipaddress
import os
import socket
import subprocess
import uuid
from pathlib import Path

try:
    import psutil
except ImportError:  # pragma: no cover - psutil is expected in production, but keep a fallback
    psutil = None


PREFERRED_INTERFACE_PREFIXES = ("en", "eth", "wlan", "wl")
DEPRIORITIZED_INTERFACE_PREFIXES = ("lo", "utun", "tun", "tap", "tailscale", "docker", "veth", "br-", "bridge", "awdl", "llw", "anpi", "ap")
EXCLUDED_IPV4_NETWORKS = (
    ipaddress.ip_network("0.0.0.0/8"),
    ipaddress.ip_network("127.0.0.0/8"),
    ipaddress.ip_network("169.254.0.0/16"),
    ipaddress.ip_network("198.18.0.0/15"),
    ipaddress.ip_network("224.0.0.0/4"),
    ipaddress.ip_network("240.0.0.0/4"),
)


def get_device_code():
    candidate = _select_device_mac_candidate()
    if candidate:
        return candidate["mac"]

    node = uuid.getnode()
    mac_hex = f"{node:012x}"
    return ":".join(mac_hex[index:index + 2] for index in range(0, 12, 2))


def get_device_fingerprint():
    for resolver in (
        _read_linux_machine_id,
        _read_linux_product_uuid,
        _read_macos_platform_uuid,
        _read_uuid_node_fingerprint,
    ):
        value = resolver()
        if value:
            return value
    return ""


def _read_linux_machine_id():
    for path in ("/etc/machine-id", "/var/lib/dbus/machine-id"):
        try:
            value = Path(path).read_text(encoding="utf-8", errors="ignore").strip().lower()
        except OSError:
            continue
        if value:
            return f"linux-machine-id:{value}"
    return ""


def _read_linux_product_uuid():
    path = Path("/sys/class/dmi/id/product_uuid")
    if not path.exists():
        return ""
    try:
        value = path.read_text(encoding="utf-8", errors="ignore").strip().lower()
    except OSError:
        return ""
    return f"linux-product-uuid:{value}" if value else ""


def _read_macos_platform_uuid():
    if os.name != "posix":
        return ""
    try:
        result = subprocess.run(
            ["ioreg", "-rd1", "-c", "IOPlatformExpertDevice"],
            capture_output=True,
            text=True,
            timeout=3,
            check=False,
        )
    except (OSError, subprocess.SubprocessError):
        return ""
    if result.returncode != 0:
        return ""
    for line in result.stdout.splitlines():
        if "IOPlatformUUID" not in line:
            continue
        _, _, raw = line.partition("=")
        value = raw.strip().strip('"').lower()
        if value:
            return f"macos-platform-uuid:{value}"
    return ""


def _read_uuid_node_fingerprint():
    node = uuid.getnode()
    if not node:
        return ""
    return f"uuid-node:{node:012x}"


def _normalize_mac(value):
    text = str(value or "").strip().lower().replace("-", ":")
    if not text:
        return None

    if ":" not in text and len(text) == 12:
        text = ":".join(text[index:index + 2] for index in range(0, 12, 2))

    parts = text.split(":")
    if len(parts) != 6:
        return None

    normalized = []
    for part in parts:
        if len(part) != 2:
            return None
        try:
            int(part, 16)
        except ValueError:
            return None
        normalized.append(part)
    mac = ":".join(normalized)
    if mac == "00:00:00:00:00:00":
        return None
    if int(normalized[0], 16) & 1:
        return None
    return mac


def _is_locally_administered_mac(mac):
    first_octet = int(str(mac).split(":")[0], 16)
    return bool(first_octet & 0b10)


def _is_physical_linux_interface(interface_name):
    root = Path("/sys/class/net")
    if not root.exists():
        return None
    try:
        resolved = root.joinpath(str(interface_name)).resolve()
    except OSError:
        return None
    return "/virtual/" not in str(resolved)


def _mac_candidate_score(interface_name, mac, *, is_up=False, ipv4_bonus=0, physical=None):
    score = 100
    name = str(interface_name or "").lower()

    if is_up:
        score += 25
    if name.startswith(PREFERRED_INTERFACE_PREFIXES):
        score += 25
    if name.startswith(DEPRIORITIZED_INTERFACE_PREFIXES):
        score -= 80
    if physical is True:
        score += 20
    elif physical is False:
        score -= 20
    if _is_locally_administered_mac(mac):
        score -= 10
    else:
        score += 10
    score += int(ipv4_bonus or 0)
    return score


def _build_psutil_interface_map():
    if psutil is None:
        return {}

    stats_by_name = psutil.net_if_stats()
    interface_map = {}
    for interface_name, items in psutil.net_if_addrs().items():
        entry = {
            "is_up": bool(getattr(stats_by_name.get(interface_name), "isup", False)),
            "mac": None,
            "ipv4_bonus": 0,
        }
        for item in items:
            mac = _normalize_mac(getattr(item, "address", ""))
            if mac and not entry["mac"]:
                entry["mac"] = mac
                continue

            if item.family != socket.AF_INET:
                continue
            address = _parse_ipv4(item.address)
            if not _is_usable_ipv4(address):
                continue
            entry["ipv4_bonus"] = max(entry["ipv4_bonus"], _candidate_score(interface_name, address, entry["is_up"]))

        if entry["mac"]:
            interface_map[str(interface_name)] = entry
    return interface_map


def _iter_psutil_mac_candidates():
    interface_map = _build_psutil_interface_map()
    candidates = []
    for interface_name, entry in interface_map.items():
        physical = _is_physical_linux_interface(interface_name)
        candidates.append(
            {
                "interface": interface_name,
                "mac": entry["mac"],
                "is_up": entry["is_up"],
                "physical": physical,
                "score": _mac_candidate_score(
                    interface_name,
                    entry["mac"],
                    is_up=entry["is_up"],
                    ipv4_bonus=entry["ipv4_bonus"],
                    physical=physical,
                ),
                "source": "psutil",
            }
        )
    return candidates


def _iter_linux_sysfs_mac_candidates():
    root = Path("/sys/class/net")
    if not root.exists():
        return []

    interface_map = _build_psutil_interface_map()
    candidates = []
    for path in root.iterdir():
        interface_name = path.name
        try:
            mac = _normalize_mac(path.joinpath("address").read_text(encoding="utf-8", errors="ignore"))
        except OSError:
            continue
        if not mac:
            continue

        entry = interface_map.get(interface_name, {})
        is_up = bool(entry.get("is_up"))
        ipv4_bonus = int(entry.get("ipv4_bonus") or 0)
        if not entry:
            try:
                is_up = path.joinpath("operstate").read_text(encoding="utf-8", errors="ignore").strip().lower() == "up"
            except OSError:
                is_up = False

        physical = _is_physical_linux_interface(interface_name)
        candidates.append(
            {
                "interface": interface_name,
                "mac": mac,
                "is_up": is_up,
                "physical": physical,
                "score": _mac_candidate_score(
                    interface_name,
                    mac,
                    is_up=is_up,
                    ipv4_bonus=ipv4_bonus,
                    physical=physical,
                ),
                "source": "linux_sysfs",
            }
        )
    return candidates


def _select_device_mac_candidate():
    candidates = _iter_psutil_mac_candidates()
    if not candidates:
        candidates = _iter_linux_sysfs_mac_candidates()
    if not candidates:
        return None

    return max(
        candidates,
        key=lambda item: (
            int(item.get("score") or 0),
            1 if item.get("physical") is True else 0,
            1 if item.get("is_up") else 0,
            str(item.get("interface") or ""),
            str(item.get("mac") or ""),
        ),
    )


def _parse_ipv4(value):
    try:
        address = ipaddress.ip_address(str(value).strip())
    except ValueError:
        return None
    if address.version != 4:
        return None
    return address


def _is_usable_ipv4(address):
    if address is None:
        return False
    if address.is_loopback or address.is_link_local or address.is_multicast or address.is_unspecified:
        return False
    return all(address not in network for network in EXCLUDED_IPV4_NETWORKS)


def _candidate_score(interface_name, address, is_up):
    score = 0
    if address.is_private:
        score += 100
    else:
        score += 60
    if is_up:
        score += 20

    name = str(interface_name or "").lower()
    if name.startswith(PREFERRED_INTERFACE_PREFIXES):
        score += 20
    if name.startswith(DEPRIORITIZED_INTERFACE_PREFIXES):
        score -= 60

    text = str(address)
    if text.startswith("192.168."):
        score += 12
    elif text.startswith("10."):
        score += 10
    elif text.startswith("172."):
        score += 8
    return score


def _iter_psutil_ipv4_candidates():
    if psutil is None:
        return []

    stats_by_name = psutil.net_if_stats()
    candidates = []
    for interface_name, items in psutil.net_if_addrs().items():
        is_up = bool(getattr(stats_by_name.get(interface_name), "isup", False))
        for item in items:
            if item.family != socket.AF_INET:
                continue
            address = _parse_ipv4(item.address)
            if not _is_usable_ipv4(address):
                continue
            candidates.append((_candidate_score(interface_name, address, is_up), str(address)))
    return candidates


def _iter_hostname_ipv4_candidates():
    candidates = []
    try:
        _, _, addresses = socket.gethostbyname_ex(socket.gethostname())
    except OSError:
        return candidates

    for value in addresses:
        address = _parse_ipv4(value)
        if not _is_usable_ipv4(address):
            continue
        candidates.append((_candidate_score("hostname", address, False), str(address)))
    return candidates


def get_local_ip():
    candidates = _iter_psutil_ipv4_candidates()
    if candidates:
        return max(candidates, key=lambda item: item[0])[1]

    candidates = _iter_hostname_ipv4_candidates()
    if candidates:
        return max(candidates, key=lambda item: item[0])[1]

    try:
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.connect(("8.8.8.8", 80))
            guessed = _parse_ipv4(sock.getsockname()[0])
            if _is_usable_ipv4(guessed):
                return str(guessed)
    except OSError:
        pass
    return "127.0.0.1"
