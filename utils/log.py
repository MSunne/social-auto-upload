import os
import threading
import time
from pathlib import Path
from sys import stdout

from loguru import logger

from conf import BASE_DIR


def _resolve_log_level():
    level = (
        os.environ.get("SAU_LOG_LEVEL")
        or getattr(__import__("conf"), "SAU_LOG_LEVEL", None)
        or "DEBUG"
    )
    return str(level).strip().upper() or "DEBUG"


LOG_LEVEL = _resolve_log_level()
LOG_ROTATION = os.environ.get("SAU_LOG_ROTATION", "20 MB")
LOG_RETENTION = os.environ.get("SAU_LOG_RETENTION", "14 days")
LOG_DIR = Path(BASE_DIR / "logs")
LOG_DIR.mkdir(parents=True, exist_ok=True)

_THROTTLE_LOCK = threading.Lock()
_THROTTLE_STATE = {}


def log_formatter(record: dict) -> str:
    colors = {
        "TRACE": "#cfe2f3",
        "INFO": "#9cbfdd",
        "DEBUG": "#8598ea",
        "WARNING": "#dcad5a",
        "SUCCESS": "#3dd08d",
        "ERROR": "#ae2c2c",
        "CRITICAL": "#ff4d4f",
    }
    color = colors.get(record["level"].name, "#b3cfe7")
    component = record["extra"].get("component") or record["extra"].get("business_name") or "app"
    return (
        f"<fg #70acde>{{time:YYYY-MM-DD HH:mm:ss.SSS}}</fg #70acde> | "
        f"<fg {color}>{{level: <8}}</fg {color}> | "
        f"<fg #7ec8e3>[{component}]</fg #7ec8e3> "
        f"<light-white>{{message}}</light-white>\n"
    )


def create_logger(log_name: str, file_path: str):
    def filter_record(record):
        return record["extra"].get("business_name") == log_name

    target_path = Path(BASE_DIR / file_path)
    target_path.parent.mkdir(parents=True, exist_ok=True)
    logger.add(
        target_path,
        filter=filter_record,
        level=LOG_LEVEL,
        rotation=LOG_ROTATION,
        retention=LOG_RETENTION,
        backtrace=True,
        diagnose=False,
        encoding="utf-8",
    )
    return logger.bind(business_name=log_name, component=log_name)


def get_logger(component: str):
    return logger.bind(component=str(component or "app").strip() or "app")


def should_log(key: str, interval_seconds: float) -> bool:
    interval = max(float(interval_seconds or 0), 0.0)
    if interval == 0:
        return True

    now = time.monotonic()
    with _THROTTLE_LOCK:
        last_logged = _THROTTLE_STATE.get(key)
        if last_logged is not None and now - last_logged < interval:
            return False
        _THROTTLE_STATE[key] = now
    return True


def log_throttled(bound_logger, level: str, key: str, interval_seconds: float, message: str, *args, **kwargs):
    if should_log(key, interval_seconds):
        bound_logger.log(str(level or "DEBUG").upper(), message, *args, **kwargs)


logger.remove()
logger.add(
    stdout,
    colorize=True,
    level=LOG_LEVEL,
    format=log_formatter,
    backtrace=True,
    diagnose=False,
)
logger.add(
    LOG_DIR / "sau.log",
    level=LOG_LEVEL,
    rotation=LOG_ROTATION,
    retention=LOG_RETENTION,
    backtrace=True,
    diagnose=False,
    encoding="utf-8",
)

app_logger = get_logger("app")
request_logger = get_logger("http")
task_logger = get_logger("publish")
login_logger = get_logger("login")
agent_logger = get_logger("agent")
ai_logger = get_logger("ai")
network_logger = get_logger("network")

douyin_logger = create_logger("douyin", "logs/douyin.log")
tencent_logger = create_logger("tencent", "logs/tencent.log")
xhs_logger = create_logger("xhs", "logs/xhs.log")
tiktok_logger = create_logger("tiktok", "logs/tiktok.log")
bilibili_logger = create_logger("bilibili", "logs/bilibili.log")
kuaishou_logger = create_logger("kuaishou", "logs/kuaishou.log")
baijiahao_logger = create_logger("baijiahao", "logs/baijiahao.log")
xiaohongshu_logger = create_logger("xiaohongshu", "logs/xiaohongshu.log")
