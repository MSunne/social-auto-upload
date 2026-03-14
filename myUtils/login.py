import asyncio
import base64
import sqlite3
import sys
import uuid
from pathlib import Path
from queue import Empty

from playwright.async_api import async_playwright

from myUtils.auth import check_cookie
from utils.base_social_media import set_init_script
from conf import BASE_DIR
from utils.browser_hook import get_browser_options

VERIFICATION_TITLE_TEXTS = [
    "身份验证",
    "验证身份",
    "安全验证",
    "登录验证",
    "验证方式",
    "选择验证方式",
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


def save_login_account(account_type, user_name, file_name, status=1):
    """同平台同账号只保留一条记录，新的 cookie 会覆盖旧记录。"""
    cookie_paths_to_delete = []

    with sqlite3.connect(Path(BASE_DIR / "db" / "database.db")) as conn:
        conn.row_factory = sqlite3.Row
        cursor = conn.cursor()
        cursor.execute(
            '''
            SELECT id, filePath
            FROM user_info
            WHERE type = ? AND userName = ?
            ORDER BY id ASC
            ''',
            (account_type, user_name)
        )
        existing_rows = cursor.fetchall()

        if existing_rows:
            primary_row = existing_rows[0]
            duplicate_ids = [row["id"] for row in existing_rows[1:]]
            cookie_paths_to_delete = [
                row["filePath"]
                for row in existing_rows
                if row["filePath"] and row["filePath"] != file_name
            ]

            cursor.execute(
                '''
                UPDATE user_info
                SET type = ?, filePath = ?, userName = ?, status = ?
                WHERE id = ?
                ''',
                (account_type, file_name, user_name, status, primary_row["id"])
            )

            if duplicate_ids:
                cursor.executemany(
                    'DELETE FROM user_info WHERE id = ?',
                    [(duplicate_id,) for duplicate_id in duplicate_ids]
                )
        else:
            cursor.execute(
                '''
                INSERT INTO user_info (type, filePath, userName, status)
                VALUES (?, ?, ?, ?)
                ''',
                (account_type, file_name, user_name, status)
            )

        conn.commit()

    for cookie_path in cookie_paths_to_delete:
        cookie_file = Path(BASE_DIR / "cookiesFile" / cookie_path)
        if cookie_file.exists():
            try:
                cookie_file.unlink()
            except OSError:
                pass


async def locator_to_data_url(locator):
    await locator.wait_for(state="visible", timeout=30000)
    image_bytes = await locator.screenshot(type="png")
    return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")


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


async def get_verification_anchor(page):
    title, title_locator = await find_first_visible_text(page, VERIFICATION_TITLE_TEXTS)
    option_texts = await collect_visible_option_texts(page)

    anchor_locator = title_locator
    if anchor_locator is None and option_texts:
        _, anchor_locator = await find_first_visible_text(page, [option_texts[0]])

    return title, option_texts, anchor_locator


async def collect_visible_option_texts(page):
    visible_texts = []
    for text in VERIFICATION_OPTION_TEXTS:
        locator = page.get_by_text(text, exact=True)
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
    return visible_texts


async def get_visible_input_hints(page):
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
            if placeholder:
                hints.append(placeholder)
            else:
                hints.append("请输入验证码或密码")
        except Exception:
            continue
    return list(dict.fromkeys(hints))


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

    if not title and not option_texts:
        return None

    input_hints = await get_visible_input_hints(page)
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


async def apply_remote_action(page, action, status_queue=None, command_queue=None):
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

    return False


async def drain_remote_actions(page, command_queue, status_queue=None):
    if command_queue is None:
        return False

    handled = False
    while True:
        try:
            action = command_queue.get_nowait()
        except Empty:
            break
        handled = await apply_remote_action(page, action, status_queue, command_queue) or handled

    return handled


async def wait_for_login_result(page, original_url, url_changed_event, status_queue, command_queue=None, timeout=200):
    deadline = asyncio.get_running_loop().time() + timeout
    last_signature = None

    while asyncio.get_running_loop().time() < deadline:
        if page.url != original_url:
            return True

        challenge = await detect_verification_challenge(page)
        if challenge:
            if challenge["signature"] != last_signature:
                push_structured_status(status_queue, command_queue, "verification_required", challenge["payload"])
                last_signature = challenge["signature"]
            handled = await drain_remote_actions(page, command_queue, status_queue)
            if handled:
                last_signature = None
                await asyncio.sleep(1)
                continue
        else:
            last_signature = None
            await drain_remote_actions(page, command_queue, status_queue)

        await asyncio.sleep(1)

    return False

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
        print("✅ 二维码已生成")
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)
        try:
            login_success = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
            )
            if not login_success:
                raise asyncio.TimeoutError
            print("监听页面跳转成功")
        except asyncio.TimeoutError:
            print("监听页面跳转超时")
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("500")
            return None
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")
        # 确保cookiesFile目录存在
        cookies_dir = Path(BASE_DIR / "cookiesFile")
        cookies_dir.mkdir(exist_ok=True)
        await context.storage_state(path=cookies_dir / f"{uuid_v1}.json")
        result = await check_cookie(3, f"{uuid_v1}.json")
        if not result:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        save_login_account(3, id, f"{uuid_v1}.json", 1)
        print("✅ 用户状态已记录")
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
        print("✅ 二维码已生成")
        status_queue.put(qr_data)

        try:
            login_success = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
            )
            if not login_success:
                raise asyncio.TimeoutError
            print("监听页面跳转成功")
        except asyncio.TimeoutError:
            status_queue.put("500")
            print("监听页面跳转超时")
            await page.close()
            await context.close()
            await browser.close()
            return None
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")
        # 确保cookiesFile目录存在
        cookies_dir = Path(BASE_DIR / "cookiesFile")
        cookies_dir.mkdir(exist_ok=True)
        await context.storage_state(path=cookies_dir / f"{uuid_v1}.json")
        result = await check_cookie(2,f"{uuid_v1}.json")
        if not result:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        save_login_account(2, id, f"{uuid_v1}.json", 1)
        print("✅ 用户状态已记录")
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
        print("✅ 二维码已生成")
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_success = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
            )
            if not login_success:
                raise asyncio.TimeoutError
            print("监听页面跳转成功")
        except asyncio.TimeoutError:
            status_queue.put("500")
            print("监听页面跳转超时")
            await page.close()
            await context.close()
            await browser.close()
            return None
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")
        # 确保cookiesFile目录存在
        cookies_dir = Path(BASE_DIR / "cookiesFile")
        cookies_dir.mkdir(exist_ok=True)
        await context.storage_state(path=cookies_dir / f"{uuid_v1}.json")
        result = await check_cookie(4, f"{uuid_v1}.json")
        if not result:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        save_login_account(4, id, f"{uuid_v1}.json", 1)
        print("✅ 用户状态已记录")
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
        print("✅ 二维码已生成")
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_success = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
            )
            if not login_success:
                raise asyncio.TimeoutError
            print("监听页面跳转成功")
        except asyncio.TimeoutError:
            status_queue.put("500")
            print("监听页面跳转超时")
            await page.close()
            await context.close()
            await browser.close()
            return None
        uuid_v1 = uuid.uuid1()
        print(f"UUID v1: {uuid_v1}")
        # 确保cookiesFile目录存在
        cookies_dir = Path(BASE_DIR / "cookiesFile")
        cookies_dir.mkdir(exist_ok=True)
        await context.storage_state(path=cookies_dir / f"{uuid_v1}.json")
        result = await check_cookie(1, f"{uuid_v1}.json")
        if not result:
            status_queue.put("500")
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        save_login_account(1, id, f"{uuid_v1}.json", 1)
        print("✅ 用户状态已记录")
        status_queue.put("200")

# a = asyncio.run(xiaohongshu_cookie_gen(4,None))
# print(a)
