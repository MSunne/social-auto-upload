import ipaddress
import socket
import uuid

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
    node = uuid.getnode()
    mac_hex = f"{node:012x}"
    return ":".join(mac_hex[index:index + 2] for index in range(0, 12, 2))


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
