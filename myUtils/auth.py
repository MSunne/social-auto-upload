import asyncio
from urllib.parse import urlparse

from playwright.async_api import async_playwright

from utils.base_social_media import set_init_script
from utils.account_storage import resolve_account_storage_state
from utils.browser_hook import get_browser_options
from utils.log import tencent_logger, kuaishou_logger, douyin_logger, xiaohongshu_logger
from utils.publish_verification import detect_publish_verification


LOGIN_PAGE_HINTS = {
    1: ["手机号登录", "扫码登录", "登录小红书", "立即登录"],
    2: ["微信扫码登录", "请使用微信扫码", "扫码登录后即可发表视频"],
    3: ["手机号登录", "扫码登录", "登录抖音创作服务平台", "验证并登录"],
    4: ["立即登录", "扫码登录", "手机号登录", "登录快手创作者服务平台"],
}

VERIFICATION_PAGE_HINTS = [
    "身份验证",
    "验证身份",
    "安全验证",
    "安全确认",
    "验证方式",
    "选择验证方式",
    "短信验证码",
    "发送验证码",
    "登录密码",
    "密码验证",
    "验证码验证",
]

SUCCESS_URL_HINTS = {
    1: {
        "hosts": ("creator.xiaohongshu.com",),
        "paths": ("/creator-micro/content/upload", "/publish/"),
    },
    2: {
        "hosts": ("channels.weixin.qq.com",),
        "paths": ("/platform/post/create", "/platform/post/list"),
    },
    3: {
        "hosts": ("creator.douyin.com",),
        "paths": ("/creator-micro/content/upload", "/creator-micro/content/post/video", "/creator-micro/content/publish"),
    },
    4: {
        "hosts": ("cp.kuaishou.com",),
        "paths": ("/article/publish/video",),
    },
}

VALIDATION_LABELS = {
    1: ("xiaohongshu", xiaohongshu_logger),
    2: ("tencent", tencent_logger),
    3: ("douyin", douyin_logger),
    4: ("kuaishou", kuaishou_logger),
}


async def has_visible_text(page, texts):
    for text in texts:
        locator = page.get_by_text(text)
        try:
            count = await locator.count()
        except Exception:
            continue
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return True
            except Exception:
                continue
    return False


def _cookie_check_result(ok, state, message, *, current_url=None):
    return {
        "ok": bool(ok),
        "state": str(state or "").strip() or ("valid" if ok else "invalid"),
        "message": str(message or "").strip() or ("cookie 有效" if ok else "cookie 已失效"),
        "currentUrl": str(current_url or "").strip() or None,
    }


async def validate_cookie_page(page, platform_type, *, settle_seconds=0.5, require_success_path=True):
    platform_label, platform_logger = VALIDATION_LABELS.get(platform_type, ("account", douyin_logger))
    if settle_seconds > 0:
        await asyncio.sleep(settle_seconds)

    try:
        verification_payload = await detect_publish_verification(page, platform_name=platform_label)
    except Exception:
        verification_payload = None
    if verification_payload:
        message = "本地 cookie 已失效，当前需要二次验证"
        platform_logger.error("[+] {}", message)
        return _cookie_check_result(False, "verification_required", message, current_url=page.url)

    if await has_visible_text(page, VERIFICATION_PAGE_HINTS):
        message = "本地 cookie 已失效，当前页面仍处于验证状态"
        platform_logger.error("[+] {}", message)
        return _cookie_check_result(False, "verification_required", message, current_url=page.url)

    if await has_visible_text(page, LOGIN_PAGE_HINTS.get(platform_type, [])):
        message = "本地 cookie 已失效，需要重新扫码登录"
        platform_logger.error("[+] {}", message)
        return _cookie_check_result(False, "login_required", message, current_url=page.url)

    current_url = str(page.url or "").strip()
    url_hints = SUCCESS_URL_HINTS.get(platform_type) or {}
    parsed_url = urlparse(current_url)
    current_host = str(parsed_url.netloc or "").strip().lower()
    current_path = str(parsed_url.path or "").strip().lower()
    expected_hosts = tuple(str(item or "").strip().lower() for item in (url_hints.get("hosts") or ()))
    expected_paths = tuple(str(item or "").strip().lower() for item in (url_hints.get("paths") or ()))
    host_matches = not expected_hosts or any(host_hint in current_host for host_hint in expected_hosts)
    path_matches = not expected_paths or any(path_hint in current_path for path_hint in expected_paths)
    if expected_hosts and not host_matches:
        message = f"本地 cookie 已失效，未进入预期站点: {current_url}"
        platform_logger.error("[+] {}", message)
        return _cookie_check_result(False, "unexpected_page", message, current_url=current_url)

    if require_success_path and expected_paths and not path_matches:
        message = f"本地 cookie 已失效，未进入预期页面: {current_url}"
        platform_logger.error("[+] {}", message)
        return _cookie_check_result(False, "unexpected_page", message, current_url=current_url)

    platform_logger.success("[+] cookie 有效")
    return _cookie_check_result(True, "valid", "本地 cookie 有效", current_url=current_url)


async def validate_active_cookie_page(page, platform_type, *, settle_seconds=0.5):
    return await validate_cookie_page(page, platform_type, settle_seconds=settle_seconds)


async def validate_login_completion_page(page, platform_type, *, settle_seconds=0.5):
    return await validate_cookie_page(
        page,
        platform_type,
        settle_seconds=settle_seconds,
        require_success_path=False,
    )


async def validate_active_tencent_page(page, *, settle_seconds=0.5):
    if settle_seconds > 0:
        await asyncio.sleep(settle_seconds)

    iframe = page.frame_locator("iframe").first
    if await has_visible_text(iframe, VERIFICATION_PAGE_HINTS):
        return _cookie_check_result(
            False,
            "verification_required",
            "本地 cookie 已失效，视频号当前仍停留在二次验证页面",
            current_url=page.url,
        )
    if await has_visible_text(iframe, LOGIN_PAGE_HINTS.get(2, [])):
        return _cookie_check_result(
            False,
            "login_required",
            "本地 cookie 已失效，视频号当前已退回登录页",
            current_url=page.url,
        )
    return await validate_cookie_page(page, 2, settle_seconds=0)


async def validate_login_completion_tencent_page(page, *, settle_seconds=0.5):
    if settle_seconds > 0:
        await asyncio.sleep(settle_seconds)

    iframe = page.frame_locator("iframe").first
    if await has_visible_text(iframe, VERIFICATION_PAGE_HINTS):
        return _cookie_check_result(
            False,
            "verification_required",
            "本地 cookie 尚未完成验证，视频号当前仍停留在二次验证页面",
            current_url=page.url,
        )
    if await has_visible_text(iframe, LOGIN_PAGE_HINTS.get(2, [])):
        return _cookie_check_result(
            False,
            "login_required",
            "本地 cookie 尚未完成登录，视频号当前仍停留在登录页",
            current_url=page.url,
        )
    return await validate_login_completion_page(page, 2, settle_seconds=0)


async def validate_active_page_detail(platform_type, page, *, settle_seconds=0.5, retries=3, retry_delay_seconds=1.0):
    attempts = max(int(retries), 1)
    last_result = None

    for attempt in range(attempts):
        if platform_type == 2:
            result = await validate_active_tencent_page(
                page,
                settle_seconds=settle_seconds if attempt == 0 else retry_delay_seconds,
            )
        else:
            result = await validate_active_cookie_page(
                page,
                platform_type,
                settle_seconds=settle_seconds if attempt == 0 else retry_delay_seconds,
            )

        last_result = result
        if result.get("ok"):
            return result

        state = str(result.get("state") or "").strip()
        if state in {"login_required", "verification_required", "missing_storage", "unsupported_platform"}:
            return result

    return last_result or _cookie_check_result(False, "error", "页面登录态校验失败", current_url=page.url)


async def validate_login_completion_detail(platform_type, page, *, settle_seconds=0.5, retries=3, retry_delay_seconds=1.0):
    attempts = max(int(retries), 1)
    last_result = None

    for attempt in range(attempts):
        if platform_type == 2:
            result = await validate_login_completion_tencent_page(
                page,
                settle_seconds=settle_seconds if attempt == 0 else retry_delay_seconds,
            )
        else:
            result = await validate_login_completion_page(
                page,
                platform_type,
                settle_seconds=settle_seconds if attempt == 0 else retry_delay_seconds,
            )

        last_result = result
        if result.get("ok"):
            return result

        state = str(result.get("state") or "").strip()
        if state in {"login_required", "verification_required", "missing_storage", "unsupported_platform"}:
            return result

    return last_result or _cookie_check_result(False, "error", "页面登录完成校验失败", current_url=page.url)


def _load_storage_state(account_ref):
    storage_state = resolve_account_storage_state(account_ref)
    if storage_state is None:
        raise FileNotFoundError(f"账号登录态不存在: {account_ref}")
    return storage_state


async def cookie_auth_douyin(account_ref, *, headless=None):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(headless=headless))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.douyin.com/creator-micro/content/upload")
            return await validate_cookie_page(page, 3)
        finally:
            await browser.close()


async def cookie_auth_tencent(account_ref, *, headless=None):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(headless=headless, extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://channels.weixin.qq.com/platform/post/create")
            return await validate_active_page_detail(2, page, settle_seconds=1.0, retries=4, retry_delay_seconds=1.0)
        finally:
            await browser.close()


async def cookie_auth_ks(account_ref, *, headless=None):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(headless=headless, extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://cp.kuaishou.com/article/publish/video")
            return await validate_cookie_page(page, 4)
        finally:
            await browser.close()


async def cookie_auth_xhs(account_ref, *, headless=None):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(headless=headless, extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.xiaohongshu.com/creator-micro/content/upload")
            return await validate_cookie_page(page, 1)
        finally:
            await browser.close()


async def check_cookie_detail(type, account_ref, *, headless=None):
    try:
        match type:
            # 小红书
            case 1:
                return await cookie_auth_xhs(account_ref, headless=headless)
            # 视频号
            case 2:
                return await cookie_auth_tencent(account_ref, headless=headless)
            # 抖音
            case 3:
                return await cookie_auth_douyin(account_ref, headless=headless)
            # 快手
            case 4:
                return await cookie_auth_ks(account_ref, headless=headless)
            case _:
                return _cookie_check_result(False, "unsupported_platform", f"不支持的平台类型: {type}")
    except FileNotFoundError:
        return _cookie_check_result(False, "missing_storage", "账号登录态不存在")
    except Exception as exc:
        return _cookie_check_result(False, "error", f"cookie 校验失败: {exc}")


async def check_cookie(type, account_ref):
    result = await check_cookie_detail(type, account_ref)
    return bool(result.get("ok"))

# a = asyncio.run(check_cookie(1,"3a6cfdc0-3d51-11f0-8507-44e51723d63c.json"))
# print(a)
