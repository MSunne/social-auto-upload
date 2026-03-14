import base64


VERIFICATION_TITLE_TEXTS = [
    "身份验证",
    "验证身份",
    "安全验证",
    "安全确认",
    "手机号验证",
    "手机验证",
    "短信验证",
    "验证码验证",
    "选择验证方式",
    "验证方式",
]

VERIFICATION_OPTION_TEXTS = [
    "接收短信验证码",
    "接收短信验证",
    "发送短信验证",
    "发送短信验证码",
    "验证登录密码",
    "登录密码",
    "密码验证",
    "继续",
    "下一步",
    "确认",
]

INPUT_HINT_KEYWORDS = [
    "验证码",
    "短信",
    "密码",
    "手机",
]


class PublishManualVerificationRequired(RuntimeError):
    def __init__(self, payload):
        super().__init__(payload.get("message") or "发布任务需要人工验证")
        self.payload = payload


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


async def collect_visible_option_texts(page):
    visible_texts = []
    for text in VERIFICATION_OPTION_TEXTS:
        locator = page.get_by_text(text, exact=True)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    visible_texts.append(text)
                    break
            except Exception:
                continue
    return visible_texts


async def collect_verification_input_hints(page):
    hints = []
    locator = page.locator("input, textarea")
    count = await locator.count()
    for index in range(min(count, 10)):
        candidate = locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
            placeholder = (await candidate.get_attribute("placeholder") or "").strip()
            if not placeholder:
                continue
            if any(keyword in placeholder for keyword in INPUT_HINT_KEYWORDS):
                hints.append(placeholder)
        except Exception:
            continue
    return list(dict.fromkeys(hints))


async def get_verification_container(anchor_locator=None):
    if anchor_locator is None:
        return None

    selectors = [
        "xpath=ancestor::*[@role='dialog' or @role='alertdialog'][1]",
        "xpath=ancestor::*[contains(@class,'dialog') or contains(@class,'modal') or contains(@class,'popup') or contains(@class,'verify')][1]",
    ]

    for selector in selectors:
        try:
            locator = anchor_locator.locator(selector)
            if await locator.count() and await locator.first.is_visible():
                return locator.first
        except Exception:
            continue
    return None


async def detect_publish_verification(page, platform_name=""):
    title, title_locator = await find_first_visible_text(page, VERIFICATION_TITLE_TEXTS)
    option_texts = await collect_visible_option_texts(page)
    input_hints = await collect_verification_input_hints(page)

    if not title and not option_texts and not input_hints:
        return None

    anchor_locator = title_locator
    if anchor_locator is None and option_texts:
        _, anchor_locator = await find_first_visible_text(page, [option_texts[0]])

    container = await get_verification_container(anchor_locator)
    screenshot_data = await page_to_data_url(page, container)
    return {
        "title": title or f"{platform_name or '平台'}需要人工验证",
        "message": f"检测到{platform_name or '平台'}发布过程需要人工验证，本条自动发布任务已终止，请在 OmniCord 中查看截图并改为人工协助发布。",
        "options": option_texts,
        "inputHints": input_hints,
        "supportsTextInput": bool(input_hints),
        "screenshotData": screenshot_data,
        "pageUrl": page.url,
    }


async def ensure_no_publish_verification(page, platform_name=""):
    payload = await detect_publish_verification(page, platform_name=platform_name)
    if payload:
        raise PublishManualVerificationRequired(payload)
