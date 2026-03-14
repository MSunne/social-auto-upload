import socket
import uuid


def get_device_code():
    node = uuid.getnode()
    mac_hex = f"{node:012x}"
    return ":".join(mac_hex[index:index + 2] for index in range(0, 12, 2))


def get_local_ip():
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
            sock.connect(("8.8.8.8", 80))
            return sock.getsockname()[0]
    except OSError:
        return "127.0.0.1"
