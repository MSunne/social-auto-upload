#!/usr/bin/env python3
import base64
import json
import os
from pathlib import Path

import requests


BASE_URL = os.environ.get("OMNIDRIVE_BASE_URL", "http://127.0.0.1:8410").rstrip("/")
API_BASE = f"{BASE_URL}/api/v1"
ADMIN_API_BASE = f"{BASE_URL}/api/admin/v1"

DEBUG_USER = {
    "email": os.environ.get("OMNIDRIVE_DEBUG_EMAIL", "debug-omnidrive@example.com"),
    "phone": os.environ.get("OMNIDRIVE_DEBUG_PHONE", "18888888888"),
    "name": os.environ.get("OMNIDRIVE_DEBUG_NAME", "Debug OmniDrive"),
    "password": os.environ.get("OMNIDRIVE_DEBUG_PASSWORD", "123456"),
}

ADMIN_USER = {
    "email": os.environ.get("OMNIDRIVE_ADMIN_EMAIL", "admin@omnidrive.local"),
    "password": os.environ.get("OMNIDRIVE_ADMIN_PASSWORD", "change-me-admin"),
}

DEVICE_CODE = os.environ.get("OMNIDRIVE_DEBUG_DEVICE_CODE", "OMNIBULL-DEBUG-001")
DEVICE_NAME = os.environ.get("OMNIDRIVE_DEBUG_DEVICE_NAME", "OmniBull Debug Node")
AGENT_KEY = os.environ.get("OMNIDRIVE_DEBUG_AGENT_KEY", "debug-omnidrive-agent-key")
MOCK_PREFIX = "[mock-seed]"

ROOT_DIR = Path(__file__).resolve().parents[1] / "omnidrive_cloud" / "data" / "mock-seed"
MATERIAL_ROOT = ROOT_DIR / "device-materials"

ONE_BY_ONE_PNG = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/w8AAusB9Y9erjQAAAAASUVORK5CYII="
)


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
        timeout=20,
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


def ensure_debug_user():
    try:
        api("POST", "/auth/register", json_payload=DEBUG_USER)
    except RuntimeError as exc:
        if "409" not in str(exc):
            raise
    login = api("POST", "/auth/login", json_payload={
        "phone": DEBUG_USER.get("phone") or DEBUG_USER["email"],
        "password": DEBUG_USER["password"],
    })
    return login["accessToken"], login["user"]


def ensure_admin_user():
    login = admin_api("POST", "/auth/login", json_payload={
        "email": ADMIN_USER["email"],
        "password": ADMIN_USER["password"],
    })
    return login["accessToken"], login["admin"]


def ensure_files():
    MATERIAL_ROOT.mkdir(parents=True, exist_ok=True)
    campaign_dir = MATERIAL_ROOT / "campaign-spring"
    campaign_dir.mkdir(parents=True, exist_ok=True)
    (campaign_dir / "campaign-brief.md").write_text(
        "# 春季上新活动\n\n- 目标平台：抖音、快手\n- 主题：轻科技家居\n- CTA：预约咨询\n",
        encoding="utf-8",
    )
    (campaign_dir / "hero-script.txt").write_text(
        "镜头 1：产品近景\n镜头 2：家庭场景\n镜头 3：品牌口号\n",
        encoding="utf-8",
    )
    (campaign_dir / "sample-video.mp4").write_bytes(b"\x00" * 2048)
    (campaign_dir / "poster.png").write_bytes(ONE_BY_ONE_PNG)
    (campaign_dir / "skill-reference.txt").write_text(
        "请生成适合短视频封面的高对比度产品主视觉。",
        encoding="utf-8",
    )
    return campaign_dir


def heartbeat_and_claim(token):
    agent_api("POST", "/agent/heartbeat", json_payload={
        "deviceCode": DEVICE_CODE,
        "deviceName": DEVICE_NAME,
        "agentKey": AGENT_KEY,
        "localIp": "127.0.0.1",
        "runtimePayload": {
            "publishTasks": {"pending": 2, "running": 1},
            "aiTasks": {"queued_cloud": 2, "publish_pending": 1},
        },
    })
    try:
        return api("POST", "/devices/claim", token=token, json_payload={"deviceCode": DEVICE_CODE})
    except RuntimeError:
        devices = api("GET", "/devices", token=token)
        device = next((item for item in devices if item["deviceCode"] == DEVICE_CODE), None)
        if not device:
            raise RuntimeError("device heartbeat/claim did not produce a visible device")
        return device


def sync_accounts():
    for platform, account_name, status in [
        ("抖音", "春季抖音号", "active"),
        ("快手", "春季快手号", "active"),
        ("视频号", "家居视频号", "inactive"),
    ]:
        agent_api("POST", "/agent/accounts/sync", json_payload={
            "deviceCode": DEVICE_CODE,
            "platform": platform,
            "accountName": account_name,
            "status": status,
            "lastMessage": None if status == "active" else "本地 cookie 待重新验证",
        })


def sync_materials(campaign_dir):
    root_payload = {
        "deviceCode": DEVICE_CODE,
        "roots": [{
            "name": "materials",
            "path": str(MATERIAL_ROOT),
            "exists": True,
            "isDirectory": True,
        }],
    }
    agent_api("POST", "/agent/materials/roots/sync", json_payload=root_payload)

    entries = []
    for entry in sorted(campaign_dir.iterdir()):
        entries.append({
            "name": entry.name,
            "kind": "directory" if entry.is_dir() else "file",
            "relativePath": entry.relative_to(MATERIAL_ROOT).as_posix(),
            "absolutePath": str(entry),
            "size": entry.stat().st_size,
            "modifiedAt": "2026-03-16 12:00:00",
            "extension": entry.suffix.lower(),
            "mimeType": {
                ".md": "text/markdown",
                ".txt": "text/plain",
                ".mp4": "video/mp4",
                ".png": "image/png",
            }.get(entry.suffix.lower(), "application/octet-stream"),
        })
    agent_api("POST", "/agent/materials/directory/sync", json_payload={
        "deviceCode": DEVICE_CODE,
        "root": "materials",
        "rootPath": str(MATERIAL_ROOT),
        "path": "campaign-spring",
        "absolutePath": str(campaign_dir),
        "entries": entries,
    })

    for entry in sorted(campaign_dir.iterdir()):
        if entry.is_dir():
            continue
        preview = None
        is_text = entry.suffix.lower() in {".md", ".txt"}
        if is_text:
            preview = entry.read_text(encoding="utf-8")
        agent_api("POST", "/agent/materials/file/sync", json_payload={
            "deviceCode": DEVICE_CODE,
            "root": "materials",
            "rootPath": str(MATERIAL_ROOT),
            "path": entry.relative_to(MATERIAL_ROOT).as_posix(),
            "absolutePath": str(entry),
            "name": entry.name,
            "size": entry.stat().st_size,
            "modifiedAt": "2026-03-16 12:00:00",
            "mimeType": {
                ".md": "text/markdown",
                ".txt": "text/plain",
                ".mp4": "video/mp4",
                ".png": "image/png",
            }.get(entry.suffix.lower(), "application/octet-stream"),
            "isText": is_text,
            "truncated": False,
            "previewText": preview,
            "extension": entry.suffix.lower(),
        })


def create_skill(token, campaign_dir):
    skill = api("POST", "/skills", token=token, json_payload={
        "name": "春季海报技能",
        "description": "用于生成适合春季上新的封面海报和短视频主视觉",
        "outputType": "image",
        "modelName": "gemini-3-pro-image-preview",
        "promptTemplate": "请根据输入文案生成高对比度、适合社媒传播的品牌主视觉。",
        "referencePayload": {"style": "bright", "campaign": "spring-launch"},
        "isEnabled": True,
    })
    with open(campaign_dir / "skill-reference.txt", "rb") as file_obj:
        api(
            "POST",
            f"/skills/{skill['id']}/upload",
            token=token,
            data={"assetType": "reference"},
            files={"file": ("skill-reference.txt", file_obj, "text/plain")},
        )
    return skill


def create_publish_task(token, device, skill):
    accounts = api("GET", "/accounts", token=token)
    target = next(item for item in accounts if item["platform"] == "抖音" and item["accountName"] == "春季抖音号")
    task = api("POST", "/tasks", token=token, json_payload={
        "deviceId": device["id"],
        "accountId": target["id"],
        "skillId": skill["id"],
        "platform": "抖音",
        "accountName": "春季抖音号",
        "title": "春季新品短视频投放",
        "contentText": "主推新品轻科技沙发，突出舒适与智能感。",
        "mediaPayload": {
            "tags": ["春季上新", "家居好物"],
            "category": "家居",
            "isDraft": False,
        },
        "materialRefs": [{
            "root": "materials",
            "path": "campaign-spring/sample-video.mp4",
            "role": "media",
        }],
    })
    return task


def create_cloud_ai_job(token, device, skill, campaign_dir):
    job = api("POST", "/ai/jobs", token=token, json_payload={
        "deviceId": device["id"],
        "skillId": skill["id"],
        "jobType": "image",
        "modelName": "gemini-3-pro-image-preview",
        "prompt": "生成一张适合春季上新活动的家居品牌海报，清新高级。",
        "inputPayload": {"ratio": "4:5", "campaign": "spring-launch"},
    })
    api("PATCH", f"/ai/jobs/{job['id']}", token=token, json_payload={
        "status": "running",
        "message": "云端开始生成海报",
    })
    with open(campaign_dir / "poster.png", "rb") as file_obj:
        api(
            "POST",
            f"/ai/jobs/{job['id']}/artifacts/upload",
            token=token,
            data={"artifactType": "image"},
            files={"file": ("poster.png", file_obj, "image/png")},
        )
    api("PATCH", f"/ai/jobs/{job['id']}", token=token, json_payload={
        "status": "completed",
        "message": "云端海报生成完成",
        "outputPayload": {"variantCount": 1},
    })
    return job


def create_local_origin_ai_job(token, device, skill, campaign_dir):
    data = agent_api("POST", "/agent/ai-jobs/sync", json_payload={
        "id": "local-ai-demo-001",
        "deviceCode": DEVICE_CODE,
        "jobType": "video",
        "modelName": "veo-3.1-fast-fl",
        "prompt": "生成一条 15 秒的春季家居广告短视频，节奏轻快。",
        "inputPayload": {"duration": 15, "aspect": "9:16"},
        "publishPayload": {
            "platformType": 3,
            "accountName": "春季抖音号",
            "accountFilePath": "mock_douyin_cookie.json",
            "title": "AI 春季家居短视频",
            "contentText": "AI 自动生成的春季新品短视频内容。",
            "tags": ["AI视频", "春季家居"],
        },
        "status": "queued_cloud",
        "message": "本地 OmniBull 已创建视频生成任务",
    })
    job = data["job"]
    with open(campaign_dir / "sample-video.mp4", "rb") as file_obj:
        api(
            "POST",
            f"/ai/jobs/{job['id']}/artifacts/upload",
            token=token,
            data={"artifactType": "video"},
            files={"file": ("sample-video.mp4", file_obj, "video/mp4")},
            params={
                "deviceId": device["id"],
                "rootName": "materials",
                "relativePath": "campaign-spring/sample-video.mp4",
            },
        )
    api("PATCH", f"/ai/jobs/{job['id']}", token=token, json_payload={
        "status": "running",
        "message": "云端开始生成视频",
    })
    api("PATCH", f"/ai/jobs/{job['id']}", token=token, json_payload={
        "status": "completed",
        "message": "云端视频生成完成，等待 OmniBull 拉取结果",
        "outputPayload": {"duration": 15},
    })
    agent_api("POST", f"/agent/ai-jobs/{job['id']}/delivery", json_payload={
        "deviceCode": DEVICE_CODE,
        "status": "publish_queued",
        "message": "模拟：AI 结果已回流 OmniBull，并排入本地发布队列",
        "localPublishTaskId": "local-publish-demo-001",
    })
    return job


def list_billing_packages(token):
    return api("GET", "/billing/packages", token=token)


def list_orders(token):
    return api("GET", "/billing/orders", token=token)


def get_order(token, order_id):
    return api("GET", f"/billing/orders/{order_id}", token=token)


def list_wallet_ledgers(token):
    return api("GET", "/billing/ledger", token=token)


def find_order_by_subject(token, subject):
    for item in list_orders(token):
        if item.get("subject") == subject:
            return item
    return None


def find_wallet_ledger_by_description(token, description):
    for item in list_wallet_ledgers(token):
        if (item.get("description") or "").strip() == description:
            return item
    return None


def pick_package_id(packages, *preferred_ids):
    package_map = {item["id"]: item for item in packages}
    for package_id in preferred_ids:
        if package_id in package_map:
            return package_id
    if not packages:
        raise RuntimeError("billing packages are empty; please bootstrap schema first")
    return packages[0]["id"]


def ensure_recharge_order(token, package_id, channel, subject):
    existing = find_order_by_subject(token, subject)
    if existing is not None:
        return get_order(token, existing["id"])
    return api("POST", "/billing/orders", token=token, json_payload={
        "packageId": package_id,
        "channel": channel,
        "subject": subject,
    })


def ensure_manual_submission(token, order, label):
    current = get_order(token, order["id"])
    if current["status"] in {"processing", "credited", "paid", "completed", "success", "rejected"}:
        return current

    submitted = api("POST", f"/billing/orders/{order['id']}/manual-submit", token=token, json_payload={
        "contactChannel": "wechat",
        "contactHandle": "mock-seed-cs",
        "paymentReference": f"MOCK-{label.upper()}",
        "transferAmountCents": current["amountCents"],
        "proofUrls": [f"https://mock.omnidrive.local/proofs/{label}.png"],
        "customerNote": f"{MOCK_PREFIX} {label} 财务联调样例",
    })
    return submitted


def ensure_support_recharge_credit(admin_token, order_id, note, payment_reference):
    detail = admin_api("GET", f"/support-recharges/{order_id}", token=admin_token)
    if detail["record"]["status"] == "credited":
        return detail
    return admin_api("POST", f"/support-recharges/{order_id}/credit", token=admin_token, json_payload={
        "note": note,
        "paymentReference": payment_reference,
    })


def ensure_support_recharge_reject(admin_token, order_id, note):
    detail = admin_api("GET", f"/support-recharges/{order_id}", token=admin_token)
    if detail["record"]["status"] == "rejected":
        return detail
    return admin_api("POST", f"/support-recharges/{order_id}/reject", token=admin_token, json_payload={
        "note": note,
    })


def ensure_wallet_adjustment(admin_token, user_id, user_token, *, amount_delta, reason, note, reference_id):
    existing = find_wallet_ledger_by_description(user_token, note)
    if existing is not None:
        return existing
    result = admin_api("POST", "/wallet-adjustments", token=admin_token, json_payload={
        "userId": user_id,
        "amountDelta": amount_delta,
        "reason": reason,
        "note": note,
        "referenceType": "mock_seed",
        "referenceId": reference_id,
        "payload": {
            "source": "seed_omnidrive_mock_data.py",
            "mock": True,
            "kind": "finance",
        },
    })
    return result["ledger"]


def seed_finance_data(user_token, user, admin_token):
    packages = list_billing_packages(user_token)
    starter_id = pick_package_id(packages, "starter")
    growth_id = pick_package_id(packages, "growth", starter_id)
    studio_id = pick_package_id(packages, "studio", growth_id, starter_id)
    enterprise_id = pick_package_id(packages, "enterprise", studio_id, growth_id, starter_id)

    alipay_pending = ensure_recharge_order(
        user_token,
        starter_id,
        "alipay",
        f"{MOCK_PREFIX} 支付宝待支付订单",
    )
    wechat_pending = ensure_recharge_order(
        user_token,
        growth_id,
        "wechatpay",
        f"{MOCK_PREFIX} 微信待支付订单",
    )
    manual_awaiting = ensure_recharge_order(
        user_token,
        studio_id,
        "manual_cs",
        f"{MOCK_PREFIX} 客服充值待提交",
    )
    manual_review = ensure_recharge_order(
        user_token,
        studio_id,
        "manual_cs",
        f"{MOCK_PREFIX} 客服充值待审核",
    )
    manual_review = ensure_manual_submission(user_token, manual_review, "pending-review")

    manual_credited = ensure_recharge_order(
        user_token,
        enterprise_id,
        "manual_cs",
        f"{MOCK_PREFIX} 客服充值已入账",
    )
    manual_credited = ensure_manual_submission(user_token, manual_credited, "credited")
    credited_detail = ensure_support_recharge_credit(
        admin_token,
        manual_credited["id"],
        f"{MOCK_PREFIX} 管理员确认入账",
        "MOCK-CREDITED-0001",
    )

    manual_rejected = ensure_recharge_order(
        user_token,
        starter_id,
        "manual_cs",
        f"{MOCK_PREFIX} 客服充值已驳回",
    )
    manual_rejected = ensure_manual_submission(user_token, manual_rejected, "rejected")
    rejected_detail = ensure_support_recharge_reject(
        admin_token,
        manual_rejected["id"],
        f"{MOCK_PREFIX} 管理员驳回样例",
    )

    wallet_compensation = ensure_wallet_adjustment(
        admin_token,
        user["id"],
        user_token,
        amount_delta=180,
        reason="前端联调补偿样例",
        note=f"{MOCK_PREFIX} 钱包补偿 +180",
        reference_id="wallet-adjustment-credit",
    )
    wallet_deduction = ensure_wallet_adjustment(
        admin_token,
        user["id"],
        user_token,
        amount_delta=-60,
        reason="前端联调扣减样例",
        note=f"{MOCK_PREFIX} 钱包扣减 -60",
        reference_id="wallet-adjustment-debit",
    )

    return {
        "orders": {
            "alipayPending": alipay_pending["id"],
            "wechatPending": wechat_pending["id"],
            "manualAwaitingSubmission": manual_awaiting["id"],
            "manualPendingReview": manual_review["id"],
            "manualCredited": credited_detail["order"]["id"],
            "manualRejected": rejected_detail["order"]["id"],
        },
        "walletLedgers": {
            "compensation": wallet_compensation["id"],
            "deduction": wallet_deduction["id"],
        },
        "summary": api("GET", "/billing/summary", token=user_token),
    }


def main():
    token, user = ensure_debug_user()
    admin_token, admin = ensure_admin_user()
    campaign_dir = ensure_files()
    device = heartbeat_and_claim(token)
    sync_accounts()
    sync_materials(campaign_dir)
    skill = create_skill(token, campaign_dir)
    publish_task = create_publish_task(token, device, skill)
    cloud_ai_job = create_cloud_ai_job(token, device, skill, campaign_dir)
    local_ai_job = create_local_origin_ai_job(token, device, skill, campaign_dir)
    finance = seed_finance_data(token, user, admin_token)

    print(json.dumps({
        "baseUrl": BASE_URL,
        "debugUser": DEBUG_USER,
        "adminUser": {
            "email": ADMIN_USER["email"],
            "name": admin["name"],
        },
        "deviceCode": DEVICE_CODE,
        "agentKey": AGENT_KEY,
        "seeded": {
            "deviceId": device["id"],
            "skillId": skill["id"],
            "publishTaskId": publish_task["id"],
            "cloudAIJobId": cloud_ai_job["id"],
            "localOriginAIJobId": local_ai_job["id"],
            "finance": finance,
        },
    }, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
