#!/usr/bin/env python3
import argparse
import base64
import json
import os
from copy import deepcopy
from datetime import datetime, timedelta, timezone
from pathlib import Path

import requests


DEFAULT_BASE_URL = os.environ.get("OMNIDRIVE_BASE_URL", "http://127.0.0.1:8410").rstrip("/")
BASE_URL = DEFAULT_BASE_URL
API_BASE = f"{BASE_URL}/api/v1"
ADMIN_API_BASE = f"{BASE_URL}/api/admin/v1"

SEED_ROOT = Path(
    os.environ.get(
        "OMNIDRIVE_DEMO_SEED_ROOT",
        str(Path(__file__).resolve().parents[1] / "omnidrive_cloud" / "data" / "mock-seed"),
    )
)
STATE_FILE = Path(os.environ.get("OMNIDRIVE_DEMO_SEED_STATE_FILE", str(SEED_ROOT / "seed-state.json")))
MATERIAL_ROOT = Path(os.environ.get("OMNIDRIVE_DEMO_MATERIAL_ROOT", str(SEED_ROOT / "device-materials")))

DEMO_USER = {
    "phone": os.environ.get("OMNIDRIVE_DEMO_PHONE", "18812345678"),
    "name": os.environ.get("OMNIDRIVE_DEMO_NAME", "禾硕AI"),
    "password": os.environ.get("OMNIDRIVE_DEMO_PASSWORD", "123456"),
}

ADMIN_USER = {
    "email": os.environ.get("OMNIDRIVE_ADMIN_EMAIL", "admin"),
    "password": os.environ.get("OMNIDRIVE_ADMIN_PASSWORD", "123456"),
}

DEVICE_CODE = os.environ.get("OMNIDRIVE_DEMO_DEVICE_CODE", "OMNIBULL-DEMO-HE-SHUO-AI")
DEVICE_NAME = os.environ.get("OMNIDRIVE_DEMO_DEVICE_NAME", "禾硕AI 演示节点")
AGENT_KEY = os.environ.get("OMNIDRIVE_DEMO_AGENT_KEY", "demo-heshuo-ai-agent-key")
ACTIVATION_CODE = os.environ.get("OMNIDRIVE_DEMO_ACTIVATION_CODE", "HSAI18812345678")
MOCK_PREFIX = "[demo-seed]"

ONE_BY_ONE_PNG = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/w8AAusB9Y9erjQAAAAASUVORK5CYII="
)
SAMPLE_VIDEO_BYTES = (b"\x00\x00\x00\x18ftypmp42" + b"\x00" * 2048)
UTC = timezone.utc


def set_runtime_paths(base_url=None, seed_root=None, state_file=None):
    global BASE_URL, API_BASE, ADMIN_API_BASE, SEED_ROOT, STATE_FILE, MATERIAL_ROOT
    if base_url:
        BASE_URL = str(base_url).rstrip("/")
        API_BASE = f"{BASE_URL}/api/v1"
        ADMIN_API_BASE = f"{BASE_URL}/api/admin/v1"
    if seed_root:
        SEED_ROOT = Path(seed_root)
        if state_file is None:
            STATE_FILE = SEED_ROOT / "seed-state.json"
        MATERIAL_ROOT = SEED_ROOT / "device-materials"
    if state_file:
        STATE_FILE = Path(state_file)


def request_api(api_base, method, path, *, token=None, json_payload=None, data=None, files=None, headers=None, params=None):
    request_headers = headers.copy() if headers else {}
    if token:
        request_headers["Authorization"] = f"Bearer {token}"
    response = requests.request(
        method=method,
        url=f"{api_base}{path}",
        headers=request_headers,
        json=json_payload,
        data=data,
        files=files,
        params=params,
        timeout=30,
    )
    if response.status_code >= 400:
        raise RuntimeError(f"{method} {path} failed: {response.status_code} {response.text}")
    if not response.content:
        return None
    return response.json()


def api(method, path, *, token=None, json_payload=None, data=None, files=None, headers=None, params=None):
    return request_api(
        API_BASE,
        method,
        path,
        token=token,
        json_payload=json_payload,
        data=data,
        files=files,
        headers=headers,
        params=params,
    )


def admin_api(method, path, *, token=None, json_payload=None, data=None, files=None, headers=None, params=None):
    return request_api(
        ADMIN_API_BASE,
        method,
        path,
        token=token,
        json_payload=json_payload,
        data=data,
        files=files,
        headers=headers,
        params=params,
    )


def agent_api(method, path, *, json_payload=None, params=None):
    return api(
        method,
        path,
        json_payload=json_payload,
        params=params,
        headers={"X-Agent-Key": AGENT_KEY},
    )


def safe_get(getter):
    try:
        return getter()
    except RuntimeError as exc:
        if " 404 " in f" {exc} ":
            return None
        raise


def load_state():
    if not STATE_FILE.exists():
        return {}
    return json.loads(STATE_FILE.read_text(encoding="utf-8"))


def save_state(state):
    STATE_FILE.parent.mkdir(parents=True, exist_ok=True)
    STATE_FILE.write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")


def state_get(state, bucket, key):
    return ((state.get(bucket) or {}).get(key) or "").strip()


def state_put(state, bucket, key, value):
    if not value:
        return
    state.setdefault(bucket, {})[key] = value


def utc_now():
    return datetime.now(UTC)


def iso_days_ago(days, *, hour=10, minute=0):
    dt = utc_now() - timedelta(days=days)
    dt = dt.replace(hour=hour, minute=minute, second=0, microsecond=0)
    return dt.isoformat().replace("+00:00", "Z")


def build_demo_account_specs():
    return [
        {"key": "douyin_home", "platform": "抖音", "accountName": "禾硕AI短视频", "status": "active", "daysAgo": 1, "lastMessage": None},
        {"key": "douyin_brand", "platform": "抖音", "accountName": "禾硕AI品牌号", "status": "active", "daysAgo": 2, "lastMessage": None},
        {"key": "douyin_local", "platform": "抖音", "accountName": "禾硕AI本地门店", "status": "active", "daysAgo": 4, "lastMessage": None},
        {"key": "kuaishou_home", "platform": "快手", "accountName": "禾硕AI快手主号", "status": "active", "daysAgo": 3, "lastMessage": None},
        {"key": "kuaishou_review", "platform": "快手", "accountName": "禾硕AI评测站", "status": "active", "daysAgo": 5, "lastMessage": None},
        {"key": "kuaishou_store", "platform": "快手", "accountName": "禾硕AI门店速推", "status": "active", "daysAgo": 7, "lastMessage": None},
        {"key": "wechat_channel", "platform": "视频号", "accountName": "禾硕AI视频号", "status": "active", "daysAgo": 6, "lastMessage": None},
        {"key": "wechat_retail", "platform": "视频号", "accountName": "禾硕AI零售观察", "status": "active", "daysAgo": 8, "lastMessage": None},
        {"key": "wechat_founder", "platform": "视频号", "accountName": "禾硕AI创始人说", "status": "active", "daysAgo": 10, "lastMessage": None},
        {"key": "xhs_official", "platform": "小红书", "accountName": "禾硕AI灵感社", "status": "active", "daysAgo": 9, "lastMessage": None},
        {"key": "xhs_casebook", "platform": "小红书", "accountName": "禾硕AI案例库", "status": "active", "daysAgo": 11, "lastMessage": None},
        {"key": "xhs_designlab", "platform": "小红书", "accountName": "禾硕AI设计实验室", "status": "active", "daysAgo": 13, "lastMessage": None},
        {"key": "bilibili_lab", "platform": "Bilibili", "accountName": "禾硕AI实验室", "status": "active", "daysAgo": 12, "lastMessage": None},
        {"key": "bilibili_ops", "platform": "Bilibili", "accountName": "禾硕AI运营课", "status": "inactive", "daysAgo": 19, "lastMessage": "近期未做本地重新校验"},
        {"key": "tiktok_global", "platform": "TikTok", "accountName": "Heshuo AI Global", "status": "active", "daysAgo": 14, "lastMessage": None},
        {"key": "tiktok_creator", "platform": "TikTok", "accountName": "Heshuo AI Creator Lab", "status": "active", "daysAgo": 16, "lastMessage": None},
        {"key": "baijiahao_news", "platform": "百家号", "accountName": "禾硕AI行业观察", "status": "active", "daysAgo": 15, "lastMessage": None},
        {"key": "baijiahao_brand", "platform": "百家号", "accountName": "禾硕AI品牌动态", "status": "inactive", "daysAgo": 22, "lastMessage": "账号长期停更，等待下一次集中校验"},
        {"key": "instagram_global", "platform": "Instagram", "accountName": "heshuo.ai.global", "status": "active", "daysAgo": 17, "lastMessage": None},
        {"key": "instagram_reels", "platform": "Instagram", "accountName": "heshuo.ai.reels", "status": "active", "daysAgo": 18, "lastMessage": None},
        {"key": "facebook_global", "platform": "Facebook", "accountName": "Heshuo AI Global", "status": "active", "daysAgo": 20, "lastMessage": None},
        {"key": "facebook_brand", "platform": "Facebook", "accountName": "Heshuo AI Brand Studio", "status": "inactive", "daysAgo": 27, "lastMessage": "海外主页素材待补齐，暂缓重新验证"},
        {"key": "youtube_growth", "platform": "YouTube", "accountName": "Heshuo AI Growth", "status": "active", "daysAgo": 21, "lastMessage": None},
        {"key": "youtube_archive", "platform": "YouTube", "accountName": "Heshuo AI Archive", "status": "inactive", "daysAgo": 30, "lastMessage": "历史频道保留中，等待统一迁移"},
    ]


def build_demo_skill_specs():
    return [
        {
            "key": "brand_image",
            "name": "禾硕AI 品牌海报",
            "description": "生成品牌海报和封面主视觉。",
            "outputType": "image",
            "modelCategory": "image",
            "promptTemplate": "请生成适合 AI 服务品牌传播的高对比度主视觉海报。",
        },
        {
            "key": "short_video",
            "name": "禾硕AI 短视频脚本",
            "description": "生成适合短视频平台的脚本与分镜。",
            "outputType": "video",
            "modelCategory": "video",
            "promptTemplate": "请生成 15 秒短视频脚本，强调转化和节奏。",
        },
        {
            "key": "channel_growth",
            "name": "禾硕AI 渠道增长文案",
            "description": "针对不同平台生成适配文案和封面建议。",
            "outputType": "image",
            "modelCategory": "image",
            "promptTemplate": "请根据平台差异生成封面图方向和标题文案。",
        },
    ]


def build_demo_image_job_specs():
    prompts = [
        "生成智能门店海报，强调到店转化。",
        "生成企业服务 KV，突出专业可信。",
        "生成直播招商封面，视觉高冲击。",
        "生成抖音投流封面，强调低门槛获客。",
        "生成小红书图文主图，强调案例感。",
        "生成视频号课程海报，强调增长方法论。",
        "生成快手家居改造前后对比封面。",
        "生成 B 站知识视频封面，强调技术感。",
        "生成 TikTok 海外投放创意海报。",
        "生成百家号行业观察配图，强调洞察。",
        "生成品牌发布会主视觉，突出科技蓝。",
        "生成门店节日促销海报，强调限时。",
        "生成产品发布长图头图，强调参数优势。",
        "生成私域引流海报，强调加微领取方案。",
        "生成达人招募海报，强调合作收益。",
        "生成正在排队的节日海报。",
        "生成正在优化的品牌海报。",
        "生成失败的风格实验海报。",
        "生成门店周年庆海报，强调现场氛围。",
        "生成海外投放促销主图，强调商品价值。",
        "生成创始人 IP 直播封面，突出人物识别。",
        "生成课程裂变活动海报，突出赠品权益。",
        "生成社群招募长图，强调入群收益。",
        "生成品牌联名预热图，突出联名合作。",
        "生成工厂探访纪实封面，强调真实可信。",
        "生成案例对比图，突出增长曲线。",
        "生成新品预约海报，强调限量名额。",
        "生成季度复盘大图，强调结果数据。",
        "生成出海广告分区主图，强调本地化风格。",
        "生成正在生成中的促销海报。",
    ]
    statuses = [
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "running", "running", "failed",
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "completed", "success", "success",
    ]
    specs = []
    for index, (prompt, status) in enumerate(zip(prompts, statuses), start=1):
        specs.append(
            {
                "key": f"image_{index:02d}",
                "jobType": "image",
                "status": status,
                "prompt": f"{MOCK_PREFIX} 图片历史 {index:02d} | {prompt}",
                "ratio": "4:5" if index % 2 else "1:1",
                "message": {
                    "success": "图片生成完成",
                    "completed": "图片生成完成",
                    "running": "图片生成中",
                    "failed": "模型风格实验失败，请稍后重试",
                }[status],
                "skillKey": "brand_image" if index <= 12 else "channel_growth",
            }
        )
    return specs


def build_demo_video_job_specs():
    prompts = [
        "生成门店引流短视频，15 秒内打透卖点。",
        "生成家居智能化演示视频，突出真实场景。",
        "生成品牌宣传视频，强调交付能力。",
        "生成服务流程解说视频，适合视频号。",
        "生成抖音带货口播视频，强调转化。",
        "生成快手案例拆解视频，强调前后对比。",
        "生成 B 站教程短视频，强调知识点。",
        "生成 TikTok 出海广告视频，强调节奏感。",
        "生成百家号资讯解读视频，强调专业。",
        "生成新品发布预热视频，强调悬念。",
        "生成招商会邀请视频，强调报名。",
        "生成企业参访记录视频，强调可信度。",
        "生成达人合作样片，强调内容质感。",
        "生成课程招生活动视频，强调报名转化。",
        "生成案例复盘视频，强调结果数据。",
        "生成正在排队的节日广告视频。",
        "生成正在运行的直播预热视频。",
        "生成正在渲染的新品预热视频。",
        "生成品牌发布会混剪视频，强调现场氛围。",
        "生成海外引流广告视频，强调节奏变化。",
        "生成直播招商预热视频，强调报名引导。",
        "生成门店改造前后对比视频，强调反差。",
        "生成创始人采访短片，强调观点输出。",
        "生成社群裂变活动视频，强调福利节奏。",
        "生成品牌联名预告视频，强调合作质感。",
        "生成客户证言视频，强调真实口碑。",
        "生成季度增长复盘视频，强调数据跃升。",
        "生成课程招生成交视频，强调转化场景。",
        "生成正在队列中的新品讲解视频。",
        "生成正在执行中的出海广告视频。",
    ]
    statuses = [
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "queued", "running", "running",
        "success", "completed", "success", "completed", "success", "completed",
        "success", "completed", "success", "completed", "success", "success",
    ]
    specs = []
    for index, (prompt, status) in enumerate(zip(prompts, statuses), start=1):
        specs.append(
            {
                "key": f"video_{index:02d}",
                "jobType": "video",
                "status": status,
                "prompt": f"{MOCK_PREFIX} 视频历史 {index:02d} | {prompt}",
                "duration": 12 + (index % 4) * 3,
                "message": {
                    "success": "视频生成完成",
                    "completed": "视频生成完成",
                    "queued": "视频任务等待云端执行",
                    "running": "视频生成中",
                }[status],
                "skillKey": "short_video",
            }
        )
    return specs


def build_demo_publish_task_specs():
    account_keys = [
        "douyin_home", "kuaishou_home", "wechat_channel", "xhs_official",
        "bilibili_lab", "tiktok_global", "baijiahao_news", "douyin_brand",
        "wechat_retail", "xhs_casebook", "kuaishou_review", "douyin_local",
        "wechat_founder", "xhs_designlab", "instagram_global", "instagram_reels",
        "facebook_global", "youtube_growth", "tiktok_creator", "baijiahao_news",
        "kuaishou_store", "douyin_home", "wechat_channel", "instagram_global",
    ]
    statuses = [
        "success", "success", "success", "success", "success", "success",
        "success", "success", "success", "success", "success", "success",
        "success", "success", "success", "success", "success", "success",
        "success", "success", "success", "pending", "running",
    ]
    titles = [
        "门店获客短视频投放", "智能家居案例发布", "课程转化视频分发", "图文种草案例发布",
        "B站知识短视频发布", "海外广告样片投放", "行业洞察视频分发", "品牌升级口播发布",
        "视频号私域引流发布", "小红书案例合集发布", "快手门店开业发布", "本地门店活动投放",
        "创始人观点短片分发", "设计实验室案例发布", "Instagram Reels 样片投放", "Instagram 海外品牌发布",
        "Facebook 海外品牌页更新", "YouTube 增长案例发布", "TikTok 创作者计划投放", "行业观察合集更新",
        "跨平台品牌整点投放", "待执行的节日短视频", "正在执行的品牌视频",
    ]
    specs = []
    for index, (account_key, status, title) in enumerate(zip(account_keys, statuses, titles), start=1):
        specs.append(
            {
                "key": f"task_{index:02d}",
                "accountKey": account_key,
                "title": f"{MOCK_PREFIX} {title}",
                "status": status,
                "message": {
                    "success": "任务执行成功",
                    "pending": "等待排期执行",
                    "running": "任务执行中",
                }[status],
                "contentText": f"第 {index} 条演示发布任务，用于展示禾硕AI 多平台发布历史。",
                "runAt": iso_days_ago(max(0, 28 - index), hour=9 + (index % 6), minute=15),
                "skillKey": "short_video" if index % 2 else "brand_image",
            }
        )
    specs.append(
        {
            "key": "task_24",
            "accountKey": "xhs_official",
            "title": f"{MOCK_PREFIX} 待人工验证的发布任务",
            "status": "needs_verify",
            "message": "发布前触发平台验证，请在本地设备继续处理。",
            "contentText": "用于展示 needs_verify 场景。",
            "runAt": iso_days_ago(0, hour=19, minute=0),
            "skillKey": "short_video",
        }
    )
    return specs


def build_demo_login_session_specs():
    return [
        {"key": "login_01", "accountKey": "douyin_home", "status": "success", "message": "扫码登录成功，本地 token 已保存。"},
        {"key": "login_02", "accountKey": "kuaishou_home", "status": "success", "message": "扫码登录成功，本地 token 已保存。"},
        {"key": "login_03", "accountKey": "wechat_channel", "status": "success", "message": "账号校验成功。"},
        {"key": "login_04", "accountKey": "xhs_official", "status": "success", "message": "小红书账号校验完成。"},
        {"key": "login_05", "accountKey": "bilibili_lab", "status": "success", "message": "B 站账号校验完成。"},
        {"key": "login_06", "accountKey": "tiktok_global", "status": "success", "message": "海外账号登录成功。"},
        {"key": "login_07", "accountKey": "instagram_global", "status": "success", "message": "Instagram 账号状态正常。"},
        {"key": "login_08", "accountKey": "youtube_growth", "status": "success", "message": "YouTube 频道登录成功。"},
        {"key": "login_09", "accountKey": "wechat_founder", "status": "verification_required", "message": "检测到登录验证，请在本地继续处理。"},
        {"key": "login_10", "accountKey": "facebook_global", "status": "failed", "message": "扫码超时，登录失败。"},
    ]


def ensure_files():
    MATERIAL_ROOT.mkdir(parents=True, exist_ok=True)
    campaign_dir = MATERIAL_ROOT / "campaign-heshuo"
    campaign_dir.mkdir(parents=True, exist_ok=True)
    (campaign_dir / "campaign-brief.md").write_text(
        "# 禾硕AI 演示活动\n\n- 目标：多平台品牌与获客演示\n- 风格：专业、可信、结果导向\n",
        encoding="utf-8",
    )
    (campaign_dir / "cover-reference.txt").write_text(
        "请突出专业感、增长结果和 AI 执行效率。",
        encoding="utf-8",
    )
    (campaign_dir / "poster.png").write_bytes(ONE_BY_ONE_PNG)
    (campaign_dir / "sample-video.mp4").write_bytes(SAMPLE_VIDEO_BYTES)
    return campaign_dir


def ensure_demo_user_exists():
    login_attempts = [
        ("/auth/login/phone", {"phone": DEMO_USER["phone"], "countryCode": "86", "password": DEMO_USER["password"]}),
        ("/auth/login/password", {"account": DEMO_USER["phone"], "password": DEMO_USER["password"]}),
    ]
    for path, payload in login_attempts:
        try:
            login = api("POST", path, json_payload=payload)
            return login["accessToken"], login["user"]
        except RuntimeError:
            continue
    raise RuntimeError(
        f"demo user {DEMO_USER['phone']} is not available; start OmniDrive in development mode so "
        "EnsureDevelopmentSeedUsers creates it before running the seed script"
    )


def ensure_admin_user():
    login = admin_api("POST", "/auth/login", json_payload={"email": ADMIN_USER["email"], "password": ADMIN_USER["password"]})
    return login["accessToken"], login["admin"]


def get_visible_device(token):
    devices = api("GET", "/devices", token=token)
    return next((item for item in devices if item["deviceCode"] == DEVICE_CODE), None)


def get_admin_device_row(admin_token):
    response = admin_api(
        "GET",
        "/devices",
        token=admin_token,
        params={"query": DEVICE_CODE, "page": 1, "pageSize": 20},
    )
    items = response.get("items") or []
    return next((item for item in items if (item.get("device") or {}).get("deviceCode") == DEVICE_CODE), None)


def heartbeat_and_claim(token, admin_token):
    agent_api(
        "POST",
        "/agent/heartbeat",
        json_payload={
            "deviceCode": DEVICE_CODE,
            "deviceName": DEVICE_NAME,
            "agentKey": AGENT_KEY,
            "localIp": "127.0.0.1",
            "runtimePayload": {
                "bridgeStatus": "healthy",
                "publishTasks": {"pending": 2, "running": 1},
                "aiTasks": {"queued_cloud": 2, "running": 2},
            },
        },
    )

    visible = get_visible_device(token)
    if visible is not None:
        return visible

    admin_row = get_admin_device_row(admin_token)
    if admin_row is None:
        raise RuntimeError("device heartbeat did not produce an admin-visible device")

    device = admin_row["device"]
    owner = admin_row.get("owner")
    if owner and owner.get("id"):
        raise RuntimeError(
            f"device {DEVICE_CODE} is already claimed by another user ({owner.get('name') or owner.get('email') or owner.get('id')})"
        )

    admin_api(
        "PATCH",
        f"/devices/{device['id']}/activation",
        token=admin_token,
        json_payload={
            "activationCode": ACTIVATION_CODE,
            "status": "ready",
            "notes": "demo seed managed activation",
        },
    )
    api("POST", "/devices/claim", token=token, json_payload={"activationCode": ACTIVATION_CODE})

    visible = get_visible_device(token)
    if visible is None:
        raise RuntimeError("device claim completed but the device is still not visible to the demo user")
    return visible


def sync_materials(campaign_dir):
    agent_api(
        "POST",
        "/agent/materials/roots/sync",
        json_payload={
            "deviceCode": DEVICE_CODE,
            "roots": [{"name": "materials", "path": str(MATERIAL_ROOT), "exists": True, "isDirectory": True}],
        },
    )

    entries = []
    for entry in sorted(campaign_dir.iterdir()):
        entries.append(
            {
                "name": entry.name,
                "kind": "directory" if entry.is_dir() else "file",
                "relativePath": entry.relative_to(MATERIAL_ROOT).as_posix(),
                "absolutePath": str(entry),
                "size": entry.stat().st_size,
                "modifiedAt": iso_days_ago(1, hour=8, minute=0).replace("T", " ").replace("Z", ""),
                "extension": entry.suffix.lower(),
                "mimeType": {
                    ".md": "text/markdown",
                    ".txt": "text/plain",
                    ".mp4": "video/mp4",
                    ".png": "image/png",
                }.get(entry.suffix.lower(), "application/octet-stream"),
            }
        )
    agent_api(
        "POST",
        "/agent/materials/directory/sync",
        json_payload={
            "deviceCode": DEVICE_CODE,
            "root": "materials",
            "rootPath": str(MATERIAL_ROOT),
            "path": campaign_dir.name,
            "absolutePath": str(campaign_dir),
            "entries": entries,
        },
    )

    for entry in sorted(campaign_dir.iterdir()):
        if entry.is_dir():
            continue
        is_text = entry.suffix.lower() in {".md", ".txt"}
        agent_api(
            "POST",
            "/agent/materials/file/sync",
            json_payload={
                "deviceCode": DEVICE_CODE,
                "root": "materials",
                "rootPath": str(MATERIAL_ROOT),
                "path": entry.relative_to(MATERIAL_ROOT).as_posix(),
                "absolutePath": str(entry),
                "name": entry.name,
                "size": entry.stat().st_size,
                "modifiedAt": iso_days_ago(1, hour=8, minute=0).replace("T", " ").replace("Z", ""),
                "mimeType": {
                    ".md": "text/markdown",
                    ".txt": "text/plain",
                    ".mp4": "video/mp4",
                    ".png": "image/png",
                }.get(entry.suffix.lower(), "application/octet-stream"),
                "isText": is_text,
                "truncated": False,
                "previewText": entry.read_text(encoding="utf-8") if is_text else None,
                "extension": entry.suffix.lower(),
            },
        )


def sync_accounts():
    for spec in build_demo_account_specs():
        payload = {
            "deviceCode": DEVICE_CODE,
            "platform": spec["platform"],
            "accountName": spec["accountName"],
            "status": spec["status"],
            "lastMessage": spec["lastMessage"],
            "lastAuthenticatedAt": iso_days_ago(spec["daysAgo"], hour=11, minute=0) if spec["status"] == "active" else None,
        }
        agent_api("POST", "/agent/accounts/sync", json_payload=payload)


def list_accounts(token):
    return api("GET", "/accounts", token=token)


def list_skills(token):
    return api("GET", "/skills", token=token)


def resolve_enabled_model(token, category, preferred):
    models = api("GET", "/ai/models", token=token)
    enabled = [item for item in models if item.get("isEnabled")]
    preferred = (preferred or "").strip()
    exact = next(
        (
            item for item in enabled
            if item.get("category") == category
            and preferred
            and preferred in {
                str(item.get("id") or "").strip(),
                str(item.get("modelName") or "").strip(),
                str(item.get("modelAlias") or "").strip(),
            }
        ),
        None,
    )
    if exact:
        return exact["modelName"]
    fallback = next((item for item in enabled if item.get("category") == category), None)
    if fallback:
        return fallback["modelName"]
    raise RuntimeError(f"no enabled AI model found for category {category}")


def resolve_video_skill_duration(token, preferred_seconds=15):
    defaults = api("GET", "/skills/defaults", token=token)
    options = defaults.get("videoTextDurationOptions") or []
    exact = next((item for item in options if item.get("durationSeconds") == preferred_seconds), None)
    if exact:
        return exact["durationSeconds"]
    if options:
        return options[0]["durationSeconds"]
    raise RuntimeError("no enabled video text duration rule found in /skills/defaults")


def ensure_skill(token, state, spec, image_model_name, video_model_name, fixed_video_duration_seconds, campaign_dir):
    logical_key = spec["key"]
    skill_id = state_get(state, "skills", logical_key)
    existing = None
    if skill_id:
        existing = safe_get(lambda: api("GET", f"/skills/{skill_id}", token=token))
    if existing is None:
        skills = list_skills(token)
        existing = next((item for item in skills if item["name"] == spec["name"]), None)
        if existing:
            skill_id = existing["id"]
            state_put(state, "skills", logical_key, skill_id)
    if existing is None:
        existing = api(
            "POST",
            "/skills",
            token=token,
            json_payload={
                "name": spec["name"],
                "description": spec["description"],
                "outputType": spec["outputType"],
                "modelName": image_model_name if spec["modelCategory"] == "image" else video_model_name,
                "fixedDurationSeconds": fixed_video_duration_seconds if spec["modelCategory"] == "video" else None,
                "promptTemplate": spec["promptTemplate"],
                "referencePayload": {"seedKey": logical_key, "source": "demo_seed"},
                "isEnabled": True,
            },
        )
        skill_id = existing["id"]
        state_put(state, "skills", logical_key, skill_id)

    assets = api("GET", f"/skills/{skill_id}/assets", token=token)
    if not assets:
        with open(campaign_dir / "cover-reference.txt", "rb") as file_obj:
            api(
                "POST",
                f"/skills/{skill_id}/upload",
                token=token,
                data={"assetType": "reference"},
                files={"file": ("cover-reference.txt", file_obj, "text/plain")},
            )
    return skill_id


def get_ai_job(token, job_id):
    return api("GET", f"/ai/jobs/{job_id}", token=token)


def list_ai_job_artifacts(token, job_id):
    return api("GET", f"/ai/jobs/{job_id}/artifacts", token=token)


def ensure_ai_artifact(token, job_id, job_type, campaign_dir):
    artifacts = list_ai_job_artifacts(token, job_id)
    if artifacts:
        return artifacts
    if job_type == "image":
        with open(campaign_dir / "poster.png", "rb") as file_obj:
            api(
                "POST",
                f"/ai/jobs/{job_id}/artifacts/upload",
                token=token,
                data={"artifactType": "image"},
                files={"file": ("poster.png", file_obj, "image/png")},
            )
    else:
        with open(campaign_dir / "sample-video.mp4", "rb") as file_obj:
            api(
                "POST",
                f"/ai/jobs/{job_id}/artifacts/upload",
                token=token,
                data={"artifactType": "video"},
                files={"file": ("sample-video.mp4", file_obj, "video/mp4")},
            )
    return list_ai_job_artifacts(token, job_id)


def ensure_ai_job(token, state, device, skill_ids, spec, image_model_name, video_model_name, campaign_dir):
    logical_key = spec["key"]
    job_id = state_get(state, "aiJobs", logical_key)
    job = None
    if job_id:
        job = safe_get(lambda: get_ai_job(token, job_id))
    if job is None:
        payload = {
            "deviceId": device["id"],
            "skillId": skill_ids.get(spec["skillKey"]),
            "jobType": spec["jobType"],
            "modelName": image_model_name if spec["jobType"] == "image" else video_model_name,
            "prompt": spec["prompt"],
            "inputPayload": {
                "seedKey": logical_key,
                "ratio": spec.get("ratio"),
                "duration": spec.get("duration"),
                "source": "demo_seed",
            },
        }
        job = api("POST", "/ai/jobs", token=token, json_payload=payload)
        job_id = job["id"]
        state_put(state, "aiJobs", logical_key, job_id)

    desired_status = spec["status"]
    current_status = (job.get("status") or "").strip()
    if desired_status in {"success", "completed", "failed"} and current_status == "queued":
        job = api("PATCH", f"/ai/jobs/{job_id}", token=token, json_payload={"status": "running", "message": "演示数据处理中"})
        current_status = job["status"]
    if desired_status in {"success", "completed"}:
        ensure_ai_artifact(token, job_id, spec["jobType"], campaign_dir)
        if current_status != desired_status:
            job = api(
                "PATCH",
                f"/ai/jobs/{job_id}",
                token=token,
                json_payload={
                    "status": desired_status,
                    "message": spec["message"],
                    "outputPayload": {"seedKey": logical_key, "status": desired_status, "source": "demo_seed"},
                },
            )
    elif current_status != desired_status:
        job = api("PATCH", f"/ai/jobs/{job_id}", token=token, json_payload={"status": desired_status, "message": spec["message"]})
    return get_ai_job(token, job_id)


def get_task(token, task_id):
    return api("GET", f"/tasks/{task_id}", token=token)


def ensure_publish_task(token, state, device, accounts_by_key, skill_ids, spec, campaign_dir):
    logical_key = spec["key"]
    task_id = state_get(state, "publishTasks", logical_key)
    task = None
    if task_id:
        task = safe_get(lambda: get_task(token, task_id))
    account = accounts_by_key[spec["accountKey"]]
    if task is None:
        task = api(
            "POST",
            "/tasks",
            token=token,
            json_payload={
                "deviceId": device["id"],
                "accountId": account["id"],
                "skillId": skill_ids.get(spec["skillKey"]),
                "platform": account["platform"],
                "accountName": account["accountName"],
                "title": spec["title"],
                "contentText": spec["contentText"],
                "mediaPayload": {"tags": ["演示数据", "禾硕AI"], "source": "demo_seed"},
                "materialRefs": [{"root": "materials", "path": f"{campaign_dir.name}/sample-video.mp4", "role": "media"}],
                "runAt": spec["runAt"],
            },
        )
        task_id = task["id"]
        state_put(state, "publishTasks", logical_key, task_id)

    if (task.get("status") or "").strip() != spec["status"] or (task.get("message") or "").strip() != spec["message"]:
        task = api(
            "PATCH",
            f"/tasks/{task_id}",
            token=token,
            json_payload={"status": spec["status"], "message": spec["message"], "runAt": spec["runAt"]},
        )
    return get_task(token, task_id)


def get_admin_login_session(admin_token, session_id):
    return admin_api("GET", f"/login-sessions/{session_id}", token=admin_token)


def ensure_login_session(admin_token, state, accounts_by_key, spec):
    logical_key = spec["key"]
    session_id = state_get(state, "loginSessions", logical_key)
    session = None
    if session_id:
        session = safe_get(lambda: get_admin_login_session(admin_token, session_id))
    if session is None:
        account = accounts_by_key[spec["accountKey"]]
        session = admin_api("POST", f"/accounts/{account['id']}/validate", token=admin_token)
        session_id = session["id"]
        state_put(state, "loginSessions", logical_key, session_id)
    if (session.get("status") or "").strip() != spec["status"]:
        session = agent_api(
            "POST",
            f"/agent/login-sessions/{session_id}/event",
            json_payload={
                "status": spec["status"],
                "message": spec["message"],
                "verificationPayload": {"seedKey": logical_key} if spec["status"] == "verification_required" else None,
            },
        )
    return safe_get(lambda: get_admin_login_session(admin_token, session_id))


def collect_account_map(token):
    items = list_accounts(token)
    key_by_target = {(spec["platform"], spec["accountName"]): spec["key"] for spec in build_demo_account_specs()}
    accounts_by_key = {}
    for item in items:
        key = key_by_target.get((item["platform"], item["accountName"]))
        if key:
            accounts_by_key[key] = item
    missing = [spec["key"] for spec in build_demo_account_specs() if spec["key"] not in accounts_by_key]
    if missing:
        raise RuntimeError(f"failed to sync demo accounts: missing {missing}")
    return accounts_by_key


def run_seed(skip_login_sessions=False):
    state = load_state()
    token, user = ensure_demo_user_exists()
    admin_token, admin = ensure_admin_user()
    campaign_dir = ensure_files()
    device = heartbeat_and_claim(token, admin_token)
    sync_materials(campaign_dir)
    sync_accounts()
    accounts_by_key = collect_account_map(token)

    image_model_name = resolve_enabled_model(token, "image", "gemini-3-pro-image-preview")
    video_model_name = resolve_enabled_model(token, "video", "veo-3.1-fast-fl")
    fixed_video_duration_seconds = resolve_video_skill_duration(token)

    skill_ids = {}
    for spec in build_demo_skill_specs():
        skill_ids[spec["key"]] = ensure_skill(
            token,
            state,
            spec,
            image_model_name,
            video_model_name,
            fixed_video_duration_seconds,
            campaign_dir,
        )

    image_jobs = []
    for spec in build_demo_image_job_specs():
        image_jobs.append(ensure_ai_job(token, state, device, skill_ids, spec, image_model_name, video_model_name, campaign_dir))

    video_jobs = []
    for spec in build_demo_video_job_specs():
        video_jobs.append(ensure_ai_job(token, state, device, skill_ids, spec, image_model_name, video_model_name, campaign_dir))

    publish_tasks = []
    for spec in build_demo_publish_task_specs():
        publish_tasks.append(ensure_publish_task(token, state, device, accounts_by_key, skill_ids, spec, campaign_dir))

    login_sessions = []
    if not skip_login_sessions:
        for spec in build_demo_login_session_specs():
            login_sessions.append(ensure_login_session(admin_token, state, accounts_by_key, spec))

    save_state(state)

    return {
        "baseUrl": BASE_URL,
        "demoUser": {"phone": DEMO_USER["phone"], "name": DEMO_USER["name"]},
        "adminUser": {"email": ADMIN_USER["email"], "name": admin["name"]},
        "device": {"id": device["id"], "deviceCode": device["deviceCode"], "agentKey": AGENT_KEY},
        "counts": {
            "accounts": len(accounts_by_key),
            "imageJobs": len(image_jobs),
            "videoJobs": len(video_jobs),
            "publishTasks": len(publish_tasks),
            "loginSessions": len(login_sessions),
        },
        "stateFile": str(STATE_FILE),
    }


def parse_args():
    parser = argparse.ArgumentParser(description="Seed rich OmniDrive demo data for 禾硕AI.")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="OmniDrive API base URL")
    parser.add_argument("--seed-root", default=str(SEED_ROOT), help="Seed working directory")
    parser.add_argument("--state-file", default=str(STATE_FILE), help="Seed state file path")
    parser.add_argument("--skip-login-sessions", action="store_true", help="Skip login session demo data")
    return parser.parse_args()


def main():
    args = parse_args()
    set_runtime_paths(base_url=args.base_url, seed_root=args.seed_root, state_file=args.state_file)
    result = run_seed(skip_login_sessions=args.skip_login_sessions)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
