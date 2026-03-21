import asyncio

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
    1: ("creator.xiaohongshu.com", "/creator-micro/content/upload", "/publish/"),
    2: ("channels.weixin.qq.com", "/platform/post/create", "/platform/post/list"),
    3: ("creator.douyin.com", "/creator-micro/content/upload", "/creator-micro/content/post/video", "/creator-micro/content/publish"),
    4: ("cp.kuaishou.com", "/article/publish/video"),
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


async def validate_cookie_page(page, platform_type):
    platform_label, platform_logger = VALIDATION_LABELS.get(platform_type, ("account", douyin_logger))
    await asyncio.sleep(2)

    try:
        verification_payload = await detect_publish_verification(page, platform_name=platform_label)
    except Exception:
        verification_payload = None
    if verification_payload:
        platform_logger.error("[+] cookie 失效，需要二次验证")
        return False

    if await has_visible_text(page, VERIFICATION_PAGE_HINTS):
        platform_logger.error("[+] cookie 失效，当前页面仍处于验证状态")
        return False

    if await has_visible_text(page, LOGIN_PAGE_HINTS.get(platform_type, [])):
        platform_logger.error("[+] cookie 失效，需要扫码登录")
        return False

    current_url = str(page.url or "").strip()
    url_hints = SUCCESS_URL_HINTS.get(platform_type, ())
    if url_hints and not any(hint in current_url for hint in url_hints):
        platform_logger.error("[+] cookie 失效，未进入预期页面 current_url={}", current_url)
        return False

    platform_logger.success("[+] cookie 有效")
    await asyncio.sleep(6)  # 保持窗口停留以充分验证
    return True


def _load_storage_state(account_ref):
    storage_state = resolve_account_storage_state(account_ref)
    if storage_state is None:
        raise FileNotFoundError(f"账号登录态不存在: {account_ref}")
    return storage_state


async def cookie_auth_douyin(account_ref):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.douyin.com/creator-micro/content/upload")
            return await validate_cookie_page(page, 3)
        finally:
            await browser.close()


async def cookie_auth_tencent(account_ref):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://channels.weixin.qq.com/platform/post/create")
            return await validate_cookie_page(page, 2)
        finally:
            await browser.close()


async def cookie_auth_ks(account_ref):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://cp.kuaishou.com/article/publish/video")
            return await validate_cookie_page(page, 4)
        finally:
            await browser.close()


async def cookie_auth_xhs(account_ref):
    storage_state = _load_storage_state(account_ref)
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options(extra_args=['--lang=en-GB']))
        try:
            context = await browser.new_context(storage_state=storage_state)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.xiaohongshu.com/creator-micro/content/upload")
            return await validate_cookie_page(page, 1)
        finally:
            await browser.close()


async def check_cookie(type, account_ref):
    try:
        match type:
            # 小红书
            case 1:
                return await cookie_auth_xhs(account_ref)
            # 视频号
            case 2:
                return await cookie_auth_tencent(account_ref)
            # 抖音
            case 3:
                return await cookie_auth_douyin(account_ref)
            # 快手
            case 4:
                return await cookie_auth_ks(account_ref)
            case _:
                return False
    except FileNotFoundError:
        return False

# a = asyncio.run(check_cookie(1,"3a6cfdc0-3d51-11f0-8507-44e51723d63c.json"))
# print(a)
