import asyncio
import base64
import sys
import uuid
from queue import Empty

from playwright.async_api import async_playwright

from myUtils.auth import check_cookie
from utils.account_storage import upsert_login_account
from utils.base_social_media import set_init_script
from utils.browser_hook import get_browser_options
from utils.log import login_logger

VERIFICATION_TITLE_TEXTS = [
    "身份验证",
    "验证身份",
    "安全验证",
    "安全确认",
    "登录验证",
    "验证方式",
    "选择验证方式",
    "选择验证方式继续登录",
    "验证你的身份",
    "验证手机号",
    "手机验证",
    "短信验证",
    "验证码验证",
]

VERIFICATION_OPTION_TEXTS = [
    "接收短信验证码",
    "接收短信验证",
    "发送短信验证",
    "发送短信验证码",
    "短信验证码",
    "验证登录密码",
    "登录密码",
    "密码验证",
    "获取验证码",
    "发送验证码",
    "短信验证",
    "手机验证",
    "验证身份",
    "下一步",
    "继续验证",
    "继续",
    "确认",
    "提交",
]

VERIFICATION_SUBMIT_TEXTS = [
    "确认",
    "提交",
    "下一步",
    "继续",
    "继续验证",
    "登录",
    "验证",
    "完成",
]

QR_EXPIRED_TEXTS = [
    "二维码已过期",
    "二维码已失效",
    "二维码失效",
    "登录二维码已过期",
    "登录二维码已失效",
    "扫码已过期",
    "扫码已失效",
    "请点击刷新",
]

QR_SCANNED_TEXTS = [
    "已扫码，请在手机上确认登录",
    "已扫码，请在手机上确认",
    "扫码成功，请在手机上确认登录",
    "扫码成功，请在手机上确认",
    "请在手机上确认登录",
    "请在手机确认登录",
    "请在手机上点击确认",
]

QR_REFRESH_TEXTS = [
    "点击刷新",
    "刷新二维码",
    "重新获取二维码",
    "重新获取",
    "重新加载",
    "点击重试",
    "刷新",
]

QR_SNAPSHOT_MIN_INTERVAL_SECONDS = 2.0
VERIFICATION_INPUT_HINT_KEYWORDS = [
    "验证码",
    "短信",
    "密码",
    "手机",
    "验证",
]


class LoginCancelled(Exception):
    pass


def save_login_account(account_type, user_name, file_name, status=1, storage_state=None):
    """同平台同账号只保留一条记录，登录态以数据库为主存。"""
    return upsert_login_account(
        account_type,
        user_name,
        file_name=file_name,
        status=status,
        storage_state=storage_state,
    )


async def locator_to_data_url(locator):
    await locator.wait_for(state="visible", timeout=30000)
    image_bytes = await locator.screenshot(type="png")
    return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")


async def locator_to_data_url_if_visible(locator):
    if locator is None:
        return None
    try:
        count = await locator.count()
        if not count:
            return None
        candidate = locator.first
        if not await candidate.is_visible():
            return None
        image_bytes = await candidate.screenshot(type="png")
        return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")
    except Exception:
        return None


async def get_locator_visual_signature_if_visible(locator):
    if locator is None:
        return None
    try:
        count = await locator.count()
        if not count:
            return None
        candidate = locator.first
        if not await candidate.is_visible():
            return None
        return await candidate.evaluate(
            """
            (element) => {
                const tag = (element.tagName || "").toLowerCase();
                if (tag === "img") {
                    return `img:${element.currentSrc || element.src || ""}`;
                }
                if (tag === "canvas") {
                    try {
                        return `canvas:${element.width || 0}x${element.height || 0}:${element.toDataURL("image/png").slice(0, 256)}`;
                    } catch (error) {
                        return `canvas:${element.width || 0}x${element.height || 0}`;
                    }
                }
                const backgroundImage = window.getComputedStyle(element).backgroundImage || "";
                const text = (element.innerText || element.textContent || "").trim();
                return `${tag}:${backgroundImage}:${text}`.slice(0, 512);
            }
            """
        )
    except Exception:
        return None


async def is_locator_visible(locator):
    if locator is None:
        return False
    try:
        count = await locator.count()
        if not count:
            return False
        return await locator.first.is_visible()
    except Exception:
        return False


async def page_to_data_url(page, locator=None):
    if locator is not None:
        try:
            if await locator.count() and await locator.first.is_visible():
                image_bytes = await locator.first.screenshot(type="png")
                return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")
        except Exception:
            pass

    image_bytes = await page.screenshot(type="png")
    return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")


async def find_first_visible_text(page, texts):
    for text in texts:
        locator = page.get_by_text(text, exact=True)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


async def find_first_visible_partial_text(page, texts):
    for text in texts:
        locator = page.get_by_text(text)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


async def find_first_visible_containing_text(target, texts):
    for text in texts:
        locator = target.get_by_text(text)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


async def get_verification_anchor(page):
    title, title_locator = await find_first_visible_text(page, VERIFICATION_TITLE_TEXTS)
    if title_locator is None:
        title, title_locator = await find_first_visible_partial_text(page, VERIFICATION_TITLE_TEXTS)
    option_texts = await collect_visible_option_texts(page)

    anchor_locator = title_locator
    if anchor_locator is None and option_texts:
        _, anchor_locator = await find_first_visible_partial_text(page, [option_texts[0]])

    if anchor_locator is None:
        page_inputs = await iter_visible_editable_inputs(page)
        if page_inputs:
            anchor_locator = page_inputs[0]

    return title, option_texts, anchor_locator


async def collect_visible_option_texts(page):
    visible_texts = []
    for text in VERIFICATION_OPTION_TEXTS:
        for locator in (page.get_by_text(text, exact=True), page.get_by_text(text)):
            count = await locator.count()
            found_visible = False
            for index in range(count):
                candidate = locator.nth(index)
                try:
                    if await candidate.is_visible():
                        found_visible = True
                        break
                except Exception:
                    continue
            if found_visible:
                visible_texts.append(text)
                break
    return list(dict.fromkeys(visible_texts))


async def get_visible_verification_input_hints(page):
    hints = []
    locator = page.locator("input, textarea")
    count = await locator.count()
    for index in range(min(count, 8)):
        candidate = locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
            input_type = (await candidate.get_attribute("type") or "text").lower()
            if input_type in {"hidden", "file", "checkbox", "radio"}:
                continue
            placeholder = (await candidate.get_attribute("placeholder") or "").strip()
            if placeholder and any(keyword in placeholder for keyword in VERIFICATION_INPUT_HINT_KEYWORDS):
                hints.append(placeholder)
                continue
            label_text = ((await candidate.get_attribute("aria-label")) or "").strip()
            if label_text and any(keyword in label_text for keyword in VERIFICATION_INPUT_HINT_KEYWORDS):
                hints.append(label_text)
                continue
            if input_type in {"password", "tel", "number"}:
                hints.append("请输入验证码或密码")
                continue
        except Exception:
            continue
    return list(dict.fromkeys(hints))


async def has_visible_verification_submit(page):
    for text in VERIFICATION_SUBMIT_TEXTS:
        _, candidate = await find_first_visible_containing_text(page, [text])
        if candidate is not None:
            return True
    return False


async def get_verification_container(page, anchor_locator=None):
    if anchor_locator is None:
        return None

    selectors = [
        "xpath=ancestor::*[@role='dialog' or @role='alertdialog'][1]",
        "xpath=ancestor::*[contains(@class,'dialog') or contains(@class,'modal') or contains(@class,'popup') or contains(@class,'verify') or contains(@class,'tooltip')][1]",
    ]

    for selector in selectors:
        try:
            locator = anchor_locator.locator(selector)
            if await locator.count() and await locator.first.is_visible():
                return locator.first
        except Exception:
            continue
    return None


async def iter_visible_editable_inputs(locator):
    try:
        candidates = locator.locator("input, textarea, [contenteditable='true']")
        count = await candidates.count()
    except Exception:
        return []

    visible_inputs = []
    for index in range(count):
        candidate = candidates.nth(index)
        try:
            if not await candidate.is_visible():
                continue
            input_type = (await candidate.get_attribute("type") or "text").lower()
            if input_type in {"hidden", "file", "checkbox", "radio"}:
                continue
            visible_inputs.append(candidate)
        except Exception:
            continue

    return visible_inputs


async def find_focused_editable_input(page):
    selectors = [
        "input:focus",
        "textarea:focus",
        "[contenteditable='true']:focus",
    ]

    for selector in selectors:
        try:
            locator = page.locator(selector)
            if await locator.count() and await locator.first.is_visible():
                return locator.first
        except Exception:
            continue

    return None


async def detect_verification_challenge(page):
    title, option_texts, anchor_locator = await get_verification_anchor(page)
    input_hints = await get_visible_verification_input_hints(page)
    has_submit = await has_visible_verification_submit(page)

    if not title and not option_texts and not input_hints:
        return None
    if not title and not option_texts and input_hints and not has_submit:
        return None
    container = await get_verification_container(page, anchor_locator)
    screenshot_data = await page_to_data_url(page, container)
    payload = {
        "title": title or "需要额外验证",
        "message": "检测到登录验证，请在远端页面选择验证方式，必要时输入验证码或密码。",
        "screenshotData": screenshot_data,
        "options": option_texts,
        "supportsTextInput": bool(input_hints),
        "inputHints": input_hints,
    }
    signature = f"{payload['title']}|{'/'.join(option_texts)}|{'/'.join(input_hints)}|{page.url}"
    return {
        "signature": signature,
        "payload": payload,
    }


async def click_visible_option(page, text):
    locator = page.get_by_text(text, exact=True)
    count = await locator.count()
    for index in range(count - 1, -1, -1):
        candidate = locator.nth(index)
        try:
            if await candidate.is_visible():
                await candidate.click(force=True, timeout=2000)
                return True
        except Exception:
            continue
    return False


async def click_visible_partial_option(target, texts):
    _, candidate = await find_first_visible_containing_text(target, texts)
    if candidate is None:
        return False
    try:
        await candidate.click(force=True, timeout=2000)
        return True
    except Exception:
        return False


async def find_first_editable_input(page):
    focused_input = await find_focused_editable_input(page)
    if focused_input is not None:
        return focused_input

    _, _, anchor_locator = await get_verification_anchor(page)
    container = await get_verification_container(page, anchor_locator)
    if container is not None:
        container_inputs = await iter_visible_editable_inputs(container)
        if container_inputs:
            return container_inputs[0]

    page_inputs = await iter_visible_editable_inputs(page)
    if page_inputs:
        return page_inputs[0]

    return None


async def click_submit_action(page):
    for text in VERIFICATION_SUBMIT_TEXTS:
        if await click_visible_option(page, text):
            return True
    return False


def get_select_all_shortcut():
    return "Meta+A" if sys.platform == "darwin" else "Control+A"


async def fill_input_like_user(page, input_locator, text):
    await input_locator.click(force=True)
    await page.keyboard.press(get_select_all_shortcut())
    await page.keyboard.press("Delete")

    try:
        await input_locator.fill(text)
        return True
    except Exception:
        try:
            await page.keyboard.type(text)
            return True
        except Exception:
            return False


def push_structured_status(status_queue, command_queue, event_type, payload):
    if status_queue is not None and command_queue is not None:
        status_queue.put({
            "type": event_type,
            "payload": payload,
        })


async def detect_login_qr_state(page, qr_locator=None, qr_action_root=None):
    qr_signature = await get_locator_visual_signature_if_visible(qr_locator)
    expired_text = None
    scanned_text = None
    for target in [target for target in (qr_action_root, page) if target is not None]:
        expired_text, _ = await find_first_visible_containing_text(target, QR_EXPIRED_TEXTS)
        if expired_text:
            break
    for target in [target for target in (qr_action_root, page) if target is not None]:
        scanned_text, _ = await find_first_visible_containing_text(target, QR_SCANNED_TEXTS)
        if scanned_text:
            break
    return {
        "qrSignature": qr_signature,
        "isExpired": bool(expired_text),
        "expiredText": expired_text,
        "isScanned": bool(scanned_text),
        "scannedText": scanned_text,
    }


async def sync_login_qr_state(status_queue, command_queue, page, qr_locator=None, qr_action_root=None, tracker=None):
    if tracker is None:
        tracker = {}

    qr_state = await detect_login_qr_state(page, qr_locator=qr_locator, qr_action_root=qr_action_root)
    now = asyncio.get_running_loop().time()
    qr_signature = qr_state.get("qrSignature")
    should_capture_qr = False
    if qr_signature and qr_signature != tracker.get("lastQrSignature"):
        should_capture_qr = True
    elif qr_signature and not tracker.get("lastQrData"):
        should_capture_qr = True
    elif qr_signature and now - float(tracker.get("lastQrSnapshotAt") or 0.0) >= QR_SNAPSHOT_MIN_INTERVAL_SECONDS:
        should_capture_qr = True

    qr_data = tracker.get("lastQrData")
    if should_capture_qr:
        latest_qr_data = await locator_to_data_url_if_visible(qr_locator)
        tracker["lastQrSnapshotAt"] = now
        if latest_qr_data:
            qr_data = latest_qr_data
            if qr_data != tracker.get("lastQrData") or qr_signature != tracker.get("lastQrSignature"):
                tracker["lastQrData"] = qr_data
                tracker["lastQrSignature"] = qr_signature
                tracker["isExpired"] = False
                tracker["isScanned"] = False
                push_structured_status(
                    status_queue,
                    command_queue,
                    "qr_updated",
                    {
                        "message": "本地登录二维码已更新，请使用最新二维码扫码。",
                        "qrData": qr_data,
                    },
                )
        elif qr_signature != tracker.get("lastQrSignature"):
            tracker["lastQrSignature"] = qr_signature

    is_expired = bool(qr_state.get("isExpired"))
    if is_expired and not tracker.get("isExpired"):
        tracker["isExpired"] = True
        push_structured_status(
            status_queue,
            command_queue,
            "qr_expired",
            {
                "message": "本地登录二维码已过期，请刷新二维码。",
                "qrData": qr_data or tracker.get("lastQrData"),
            },
        )
    elif not is_expired and tracker.get("isExpired") and qr_data:
        tracker["isExpired"] = False

    is_scanned = bool(qr_state.get("isScanned"))
    if is_scanned and not tracker.get("isScanned"):
        tracker["isScanned"] = True
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {
                "message": qr_state.get("scannedText") or "已扫码，请在手机上确认登录。",
            },
        )
    elif not is_scanned and tracker.get("isScanned"):
        tracker["isScanned"] = False

    return qr_state


async def apply_remote_action(page, action, status_queue=None, command_queue=None, qr_action_root=None):
    if not action:
        return False

    action_type = action.get("actionType") or ""
    payload = action.get("payload") or {}

    if action_type in {"select_option", "click_text"}:
        target_text = str(payload.get("text") or payload.get("optionText") or "").strip()
        if not target_text:
            return False
        result = await click_visible_option(page, target_text)
        if result:
            push_structured_status(
                status_queue,
                command_queue,
                "log",
                {"message": f"远端已选择验证方式：{target_text}"},
            )
        return result

    if action_type == "fill_text":
        text = str(payload.get("text") or "").strip()
        if not text:
            return False
        input_locator = await find_first_editable_input(page)
        if input_locator is None:
            return False
        filled = await fill_input_like_user(page, input_locator, text)
        if not filled:
            return False
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": "远端已向本地验证框填入内容"},
        )
        return True

    if action_type == "press_key":
        key = str(payload.get("key") or "Enter").strip() or "Enter"
        input_locator = await find_first_editable_input(page)
        if input_locator is not None:
            try:
                await input_locator.click(force=True)
                await input_locator.press(key)
            except Exception:
                await page.keyboard.press(key)
        else:
            await page.keyboard.press(key)
        if key.lower() == "enter":
            await asyncio.sleep(0.2)
            await click_submit_action(page)
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": f"远端已向本地浏览器发送按键：{key}"},
        )
        return True

    if action_type == "fill_text_and_submit":
        text = str(payload.get("text") or "").strip()
        if not text:
            return False
        input_locator = await find_first_editable_input(page)
        if input_locator is None:
            return False
        filled = await fill_input_like_user(page, input_locator, text)
        if not filled:
            return False
        await asyncio.sleep(0.2)
        try:
            await input_locator.press("Enter")
        except Exception:
            await page.keyboard.press("Enter")
        await asyncio.sleep(0.2)
        await click_submit_action(page)
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": "远端已发送验证码并尝试提交验证"},
        )
        return True

    if action_type == "refresh_qr":
        targets = [target for target in (qr_action_root, page) if target is not None]
        for target in targets:
            refreshed = await click_visible_partial_option(target, QR_REFRESH_TEXTS)
            if refreshed:
                push_structured_status(
                    status_queue,
                    command_queue,
                    "log",
                    {"message": "远端已请求本地 SAU 刷新二维码"},
                )
                return True
        return False

    if action_type in {"cancel_session", "cancel_login"}:
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": "远端已取消当前登录会话，本地 SAU 正在关闭登录窗口。"},
        )
        try:
            await page.close()
        except Exception:
            pass
        raise LoginCancelled()

    return False


async def drain_remote_actions(page, command_queue, status_queue=None, qr_action_root=None):
    if command_queue is None:
        return False

    handled = False
    while True:
        try:
            action = command_queue.get_nowait()
        except Empty:
            break
        handled = await apply_remote_action(page, action, status_queue, command_queue, qr_action_root=qr_action_root) or handled

    return handled


async def wait_for_login_result(
    page,
    original_url,
    url_changed_event,
    status_queue,
    command_queue=None,
    timeout=200,
    qr_locator=None,
    qr_action_root=None,
    initial_qr_data=None,
    verification_timeout=900,
    verification_settle_seconds=2.0,
):
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    last_signature = None
    qr_tracker = {"lastQrData": initial_qr_data, "isExpired": False, "isScanned": False}
    qr_hidden_since = None
    verification_started_at = None
    verification_cleared_since = None

    while loop.time() < deadline:
        now = loop.time()
        if page.is_closed():
            login_logger.info("login page closed after qr flow original_url={}", original_url)
            return "cancelled"

        qr_state = await sync_login_qr_state(
            status_queue,
            command_queue,
            page,
            qr_locator=qr_locator,
            qr_action_root=qr_action_root,
            tracker=qr_tracker,
        )
        qr_visible = await is_locator_visible(qr_locator)
        qr_phase_waiting_scan = bool(
            qr_locator is not None and qr_visible and not qr_state.get("isScanned") and not qr_state.get("isExpired")
        )

        challenge = None if qr_phase_waiting_scan else await detect_verification_challenge(page)
        if challenge:
            if verification_started_at is None:
                verification_started_at = now
                deadline = max(deadline, now + max(verification_timeout, timeout))
                login_logger.info(
                    "login verification challenge detected original_url={} current_url={} timeout_extended_to={}s",
                    original_url,
                    page.url,
                    max(verification_timeout, timeout),
                )
            verification_cleared_since = None
            if challenge["signature"] != last_signature:
                push_structured_status(status_queue, command_queue, "verification_required", challenge["payload"])
                last_signature = challenge["signature"]
            try:
                handled = await drain_remote_actions(page, command_queue, status_queue, qr_action_root=qr_action_root)
            except LoginCancelled:
                login_logger.info("login cancelled during verification original_url={} current_url={}", original_url, page.url)
                return "cancelled"
            if handled:
                deadline = max(deadline, loop.time() + 180)
                last_signature = None
                await asyncio.sleep(0.5)
                continue
            await asyncio.sleep(0.5)
            continue

        verification_active = verification_started_at is not None
        if verification_active and verification_cleared_since is None:
            verification_cleared_since = now
            login_logger.info(
                "login verification challenge cleared original_url={} current_url={} waiting_for_completion=true",
                original_url,
                page.url,
            )
        verification_settled = not verification_active or (
            verification_cleared_since is not None and now - verification_cleared_since >= verification_settle_seconds
        )

        if url_changed_event.is_set() or page.url != original_url:
            if not verification_settled:
                await asyncio.sleep(0.5)
                continue
            login_logger.info("login navigation detected original_url={} current_url={}", original_url, page.url)
            return True

        if qr_locator is not None and not qr_visible and not qr_state.get("isExpired"):
            if qr_state.get("isScanned"):
                deadline = max(deadline, loop.time() + 180)
                qr_hidden_since = None
            elif not verification_settled:
                qr_hidden_since = None
            elif qr_hidden_since is None:
                qr_hidden_since = now
            elif now - qr_hidden_since >= 1.5:
                login_logger.info("login qr disappeared and stayed hidden original_url={} current_url={}", original_url, page.url)
                return True
        else:
            qr_hidden_since = None

        last_signature = None
        try:
            handled = await drain_remote_actions(page, command_queue, status_queue, qr_action_root=qr_action_root)
        except LoginCancelled:
            login_logger.info("login cancelled original_url={} current_url={}", original_url, page.url)
            return "cancelled"
        if handled:
            if verification_active:
                deadline = max(deadline, loop.time() + 180)
            await asyncio.sleep(0.5)
            continue

        if verification_active and verification_settled:
            verification_started_at = None
            verification_cleared_since = None

        await asyncio.sleep(0.5)

    return False


async def persist_login_state_with_retry(
    context,
    account_type,
    account_name,
    platform_label,
    verify_timeout=30,
    verify_interval=2,
    page=None,
    status_queue=None,
    command_queue=None,
):
    uuid_v1 = uuid.uuid1()
    file_name = f"{uuid_v1}.json"
    settle_deadline = asyncio.get_running_loop().time() + 1.5
    while asyncio.get_running_loop().time() < settle_deadline:
        await drain_remote_actions(page, command_queue, status_queue)
        await asyncio.sleep(0.2)

    deadline = asyncio.get_running_loop().time() + max(verify_timeout, verify_interval)
    attempt = 0
    last_error = None
    last_verification_signature = None

    while asyncio.get_running_loop().time() < deadline:
        attempt += 1
        await drain_remote_actions(page, command_queue, status_queue)
        if page is not None and page.is_closed():
            raise LoginCancelled()
        if page is not None and not page.is_closed():
            challenge = await detect_verification_challenge(page)
            if challenge:
                if challenge["signature"] != last_verification_signature:
                    push_structured_status(status_queue, command_queue, "verification_required", challenge["payload"])
                    last_verification_signature = challenge["signature"]
                await asyncio.sleep(0.5)
                continue
        last_verification_signature = None
        try:
            storage_state = await context.storage_state()
            if await check_cookie(account_type, storage_state):
                save_login_account(account_type, account_name, file_name, 1, storage_state=storage_state)
                login_logger.info(
                    "{} login cookie saved account_name={} cookie_file={} attempts={}",
                    platform_label,
                    account_name,
                    file_name,
                    attempt,
                )
                return file_name
            last_error = "cookie_invalid"
            login_logger.warning(
                "{} login detected but cookie not ready account_name={} attempt={}",
                platform_label,
                account_name,
                attempt,
            )
        except Exception as exc:
            last_error = str(exc)
            login_logger.warning(
                "{} login cookie verify error account_name={} attempt={} error={}",
                platform_label,
                account_name,
                attempt,
                exc,
            )

        sleep_deadline = asyncio.get_running_loop().time() + verify_interval
        while asyncio.get_running_loop().time() < sleep_deadline:
            await drain_remote_actions(page, command_queue, status_queue)
            await asyncio.sleep(0.2)

    login_logger.error(
        "{} login cookie verify failed account_name={} attempts={} last_error={}",
        platform_label,
        account_name,
        attempt,
        last_error,
    )
    return None

# 抖音登录
async def douyin_cookie_gen(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()
    async with async_playwright() as playwright:
        options = get_browser_options()
        # Make sure to run headed.
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://creator.douyin.com/")
        original_url = page.url
        img_locator = page.get_by_role("img", name="二维码")
        qr_data = await locator_to_data_url(img_locator)
        login_logger.info("douyin qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)
        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
            )
            if login_result == "cancelled":
                login_logger.info("douyin login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("douyin login navigation detected account_name={}", id)
        except asyncio.TimeoutError:
            login_logger.warning("douyin login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("500")
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                3,
                id,
                "douyin",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("douyin login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        if not saved_file:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")


# 视频号登录
async def get_tencent_cookie(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()

    async with async_playwright() as playwright:
        options = get_browser_options(extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        # Pause the page, and start recording manually.
        context = await set_init_script(context)
        page = await context.new_page()
        await page.goto("https://channels.weixin.qq.com")
        original_url = page.url

        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        # 等待 iframe 出现（最多等 60 秒）
        iframe_locator = page.frame_locator("iframe").first

        # 获取 iframe 中的第一个 img 元素
        img_locator = iframe_locator.get_by_role("img").first

        qr_data = await locator_to_data_url(img_locator)
        login_logger.info("tencent qr generated account_name={}", id)
        status_queue.put(qr_data)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=iframe_locator,
                initial_qr_data=qr_data,
            )
            if login_result == "cancelled":
                login_logger.info("tencent login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("tencent login navigation detected account_name={}", id)
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("tencent login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                2,
                id,
                "tencent",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("tencent login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        if not saved_file:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# 快手登录
async def get_ks_cookie(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()
    async with async_playwright() as playwright:
        options = get_browser_options(extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://cp.kuaishou.com")

        # 定位并点击“立即登录”按钮（类型为 link）
        await page.get_by_role("link", name="立即登录").click()
        await page.get_by_text("扫码登录").click()
        img_locator = page.get_by_role("img", name="qrcode")
        qr_data = await locator_to_data_url(img_locator)
        original_url = page.url
        login_logger.info("kuaishou qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
            )
            if login_result == "cancelled":
                login_logger.info("kuaishou login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("kuaishou login navigation detected account_name={}", id)
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("kuaishou login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                4,
                id,
                "kuaishou",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("kuaishou login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        if not saved_file:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# 小红书登录
async def xiaohongshu_cookie_gen(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()

    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()

    async with async_playwright() as playwright:
        options = get_browser_options(extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://creator.xiaohongshu.com/")
        await page.locator('img.css-wemwzq').click()

        img_locator = page.get_by_role("img").nth(2)
        qr_data = await locator_to_data_url(img_locator)
        original_url = page.url
        login_logger.info("xiaohongshu qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
            )
            if login_result == "cancelled":
                login_logger.info("xiaohongshu login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("xiaohongshu login navigation detected account_name={}", id)
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("xiaohongshu login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                1,
                id,
                "xiaohongshu",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("xiaohongshu login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        if not saved_file:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# a = asyncio.run(xiaohongshu_cookie_gen(4,None))
# print(a)
