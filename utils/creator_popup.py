from playwright.async_api import Error as PlaywrightError


POPUP_TEXTS = {
    "douyin": ["我知道了", "知道了"],
    "tencent": ["我知道了", "知道了", "跳过", "暂不体验"],
    "kuaishou": ["我知道了", "知道了", "跳过"],
}

POPUP_NEXT_TEXTS = {
    "douyin": [],
    "tencent": ["下一步", "完成"],
    "kuaishou": ["下一步", "完成"],
}

POPUP_CLOSE_SELECTORS = {
    "douyin": [
        ".el-dialog__close",
        "[aria-label='关闭']",
        "[aria-label='close']",
        "svg[style*='cursor: pointer']",
    ],
    "tencent": [
        ".el-dialog__close",
        "[aria-label='关闭']",
        "[aria-label='close']",
        "[data-action='skip']",
        "[title='Skip']",
        "svg[style*='cursor: pointer']",
    ],
    "kuaishou": [
        ".el-dialog__close",
        "[data-action='skip']",
        "[aria-label='关闭']",
        "[aria-label='Skip']",
        "[aria-label='close']",
        "[title='Skip']",
        "svg[style*='cursor: pointer']",
    ],
}


async def click_visible_texts(page, texts):
    clicks = 0

    for text in texts:
        try:
            locator = page.get_by_text(text, exact=True)
            count = await locator.count()
        except PlaywrightError:
            continue

        for index in range(count - 1, -1, -1):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    await candidate.click(force=True, timeout=1500)
                    clicks += 1
                    await page.wait_for_timeout(250)
            except PlaywrightError:
                continue

    return clicks


async def click_visible_selectors(page, selectors):
    for selector in selectors:
        try:
            locator = page.locator(selector)
            count = await locator.count()
        except PlaywrightError:
            continue

        for index in range(count - 1, -1, -1):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    await candidate.click(force=True, timeout=1500)
                    await page.wait_for_timeout(250)
                    return 1
            except PlaywrightError:
                continue

    return 0


async def dismiss_platform_popups(page, platform, max_rounds=6):
    platform = platform or ""
    total_clicks = 0

    for _ in range(max_rounds):
        round_clicks = 0
        round_clicks += await click_visible_texts(page, POPUP_TEXTS.get(platform, []))

        if round_clicks == 0:
            round_clicks += await click_visible_selectors(page, POPUP_CLOSE_SELECTORS.get(platform, []))

        if round_clicks == 0:
            round_clicks += await click_visible_texts(page, POPUP_NEXT_TEXTS.get(platform, []))

        if round_clicks == 0:
            break

        total_clicks += round_clicks
        await page.wait_for_timeout(400)

    return total_clicks
