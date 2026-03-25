# -*- coding: utf-8 -*-
from datetime import datetime

from playwright.async_api import Playwright, async_playwright, Page
import asyncio

from conf import LOCAL_CHROME_HEADLESS
from utils.account_storage import account_storage_exists, load_account_storage_state, update_account_storage_state
from utils.base_social_media import set_init_script
from utils.browser_hook import get_browser_options
from utils.creator_popup import dismiss_platform_popups
from utils.log import douyin_logger
from utils.publish_verification import PublishManualVerificationRequired, ensure_no_publish_verification


async def cookie_auth(account_file):
    storage_state = load_account_storage_state(account_file)
    if storage_state is None:
        return False
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        context = await browser.new_context(storage_state=storage_state)
        context = await set_init_script(context)
        # 创建一个新的页面
        page = await context.new_page()
        # 访问指定的 URL
        await page.goto("https://creator.douyin.com/creator-micro/content/upload")
        try:
            await page.wait_for_url("https://creator.douyin.com/creator-micro/content/upload", timeout=5000)
        except:
            print("[+] 等待5秒 cookie 失效")
            await context.close()
            await browser.close()
            return False
        # 2024.06.17 抖音创作者中心改版
        if await page.get_by_text('手机号登录').count() or await page.get_by_text('扫码登录').count():
            print("[+] 等待5秒 cookie 失效")
            return False
        else:
            print("[+] cookie 有效")
            return True


async def douyin_setup(account_file, handle=False):
    if not account_storage_exists(account_file) or not await cookie_auth(account_file):
        if not handle:
            # Todo alert message
            return False
        douyin_logger.info('[+] cookie文件不存在或已失效，即将自动打开浏览器，请扫码登录，登陆后会自动生成cookie文件')
        await douyin_cookie_gen(account_file)
    return True


async def douyin_cookie_gen(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://creator.douyin.com/")
        await page.pause()
        # 点击调试器的继续，保存cookie
        update_account_storage_state(account_file, await context.storage_state())


class DouYinVideo(object):
    def __init__(self, title, file_path, tags, publish_date: datetime, account_file, thumbnail_path=None, productLink='', productTitle=''):
        self.title = title  # 视频标题
        self.file_path = file_path
        self.tags = tags
        self.publish_date = publish_date
        self.account_file = account_file
        self.date_format = '%Y年%m月%d日 %H:%M'
        self.headless = LOCAL_CHROME_HEADLESS
        self.thumbnail_path = thumbnail_path
        self.productLink = productLink
        self.productTitle = productTitle

    async def set_schedule_time_douyin(self, page, publish_date):
        # 选择包含特定文本内容的 label 元素
        label_element = page.locator("[class^='radio']:has-text('定时发布')")
        # 在选中的 label 元素下点击 checkbox
        await label_element.click()
        await asyncio.sleep(1)
        publish_date_hour = publish_date.strftime("%Y-%m-%d %H:%M")

        await asyncio.sleep(1)
        await page.locator('.semi-input[placeholder="日期和时间"]').click()
        await page.keyboard.press("Control+KeyA")
        await page.keyboard.type(str(publish_date_hour))
        await page.keyboard.press("Enter")

        await asyncio.sleep(1)

    async def handle_upload_error(self, page):
        douyin_logger.info('视频出错了，重新上传中')
        await page.locator('div.progress-div [class^="upload-btn-input"]').set_input_files(self.file_path)

    async def upload(self, playwright: Playwright) -> None:
        # 使用 Chromium 浏览器启动一个浏览器实例
        browser = await playwright.chromium.launch(**get_browser_options(headless=self.headless))
        # 创建一个浏览器上下文，使用指定的 cookie 文件
        storage_state = load_account_storage_state(self.account_file)
        if storage_state is None:
            await browser.close()
            raise FileNotFoundError(f"账号登录态不存在: {self.account_file}")
        context = await browser.new_context(storage_state=storage_state)
        context = await set_init_script(context)

        try:
            # 创建一个新的页面
            page = await context.new_page()
            # 访问指定的 URL
            await page.goto("https://creator.douyin.com/creator-micro/content/upload")
            douyin_logger.info(f'[+]正在上传-------{self.title}.mp4')
            # 等待页面跳转到指定的 URL，没进入，则自动等待到超时
            douyin_logger.info(f'[-] 正在打开主页...')
            await page.wait_for_url("https://creator.douyin.com/creator-micro/content/upload")
            await dismiss_platform_popups(page, "douyin")
            await ensure_no_publish_verification(page, "抖音")
            # 点击 "上传视频" 按钮
            await page.locator("div[class^='container'] input").set_input_files(self.file_path)

            # 等待页面跳转到指定的 URL 2025.01.08修改在原有基础上兼容两种页面
            while True:
                try:
                    await page.wait_for_url(
                        "https://creator.douyin.com/creator-micro/content/publish?enter_from=publish_page", timeout=3000)
                    douyin_logger.info("[+] 成功进入version_1发布页面!")
                    break
                except Exception:
                    try:
                        await page.wait_for_url(
                            "https://creator.douyin.com/creator-micro/content/post/video?enter_from=publish_page",
                            timeout=3000)
                        douyin_logger.info("[+] 成功进入version_2发布页面!")
                        break
                    except Exception:
                        print("  [-] 超时未进入视频发布页面，重新尝试...")
                        await asyncio.sleep(0.5)
            await dismiss_platform_popups(page, "douyin")
            await ensure_no_publish_verification(page, "抖音")
            await asyncio.sleep(1)
            douyin_logger.info(f'  [-] 正在填充标题和话题...')
            title_container = page.get_by_text('作品标题').locator("..").locator("xpath=following-sibling::div[1]").locator("input")
            if await title_container.count():
                await title_container.fill(self.title[:30])
            else:
                titlecontainer = page.locator(".notranslate")
                await titlecontainer.click()
                await page.keyboard.press("Backspace")
                await page.keyboard.press("Control+KeyA")
                await page.keyboard.press("Delete")
                await page.keyboard.type(self.title)
                await page.keyboard.press("Enter")
            css_selector = ".zone-container"
            for index, tag in enumerate(self.tags, start=1):
                await page.type(css_selector, "#" + tag)
                await page.press(css_selector, "Space")
            douyin_logger.info(f'总共添加{len(self.tags)}个话题')
            while True:
                try:
                    number = await page.locator('[class^="long-card"] div:has-text("重新上传")').count()
                    if number > 0:
                        douyin_logger.success("  [-]视频上传完毕")
                        break
                    douyin_logger.info("  [-] 正在上传视频中...")
                    await asyncio.sleep(2)
                    await ensure_no_publish_verification(page, "抖音")

                    if await page.locator('div.progress-div > div:has-text("上传失败")').count():
                        douyin_logger.error("  [-] 发现上传出错了... 准备重试")
                        await self.handle_upload_error(page)
                except PublishManualVerificationRequired:
                    raise
                except Exception:
                    douyin_logger.info("  [-] 正在上传视频中...")
                    await asyncio.sleep(2)

            if self.productLink and self.productTitle:
                douyin_logger.info(f'  [-] 正在设置商品链接...')
                await self.set_product_link(page, self.productLink, self.productTitle)
                douyin_logger.info(f'  [+] 完成设置商品链接...')

            await self.set_thumbnail(page, self.thumbnail_path)
            await self.set_location(page, "")
            if not self.thumbnail_path:
                await self.handle_auto_video_cover(page)

            third_part_element = '[class^="info"] > [class^="first-part"] div div.semi-switch'
            if await page.locator(third_part_element).count():
                if 'semi-switch-checked' not in await page.eval_on_selector(third_part_element, 'div => div.className'):
                    await page.locator(third_part_element).locator('input.semi-switch-native-control').click()

            if self.publish_date != 0:
                await self.set_schedule_time_douyin(page, self.publish_date)

            while True:
                try:
                    await dismiss_platform_popups(page, "douyin")
                    await ensure_no_publish_verification(page, "抖音")
                    publish_button = page.get_by_role('button', name="发布", exact=True)
                    if await publish_button.count():
                        await publish_button.click()
                    await page.wait_for_url("https://creator.douyin.com/creator-micro/content/manage**", timeout=3000)
                    douyin_logger.success("  [-]视频发布成功")
                    break
                except PublishManualVerificationRequired:
                    raise
                except Exception:
                    await dismiss_platform_popups(page, "douyin")
                    await ensure_no_publish_verification(page, "抖音")
                    await self.handle_auto_video_cover(page)
                    douyin_logger.info("  [-] 视频正在发布中...")
                    await page.screenshot(full_page=True)
                    await asyncio.sleep(0.5)

            update_account_storage_state(self.account_file, await context.storage_state())
            douyin_logger.success('  [-]cookie更新完毕！')
            await asyncio.sleep(2)
        finally:
            await context.close()
            await browser.close()

    async def _find_first_visible_locator(self, locators):
        for locator in locators:
            try:
                count = await locator.count()
            except Exception:
                continue
            for index in range(count):
                candidate = locator.nth(index)
                try:
                    if await candidate.is_visible():
                        return candidate
                except Exception:
                    continue
        return None

    async def _find_recommend_cover(self, page):
        return await self._find_first_visible_locator(
            [
                page.locator('[class^="recommendCover-"]'),
                page.locator('[class*="recommendCover"]'),
                page.locator('[class*="coverItem"]'),
                page.locator('[class*="cover-item"]'),
                page.locator('[class*="cover"] [role="radio"]'),
                page.locator('[class*="cover"] [role="option"]'),
                page.locator('[class*="cover"] img'),
            ]
        )

    async def _find_cover_modal(self, page):
        return await self._find_first_visible_locator(
            [
                page.locator("div.dy-creator-content-modal"),
                page.locator('div[role="dialog"]'),
                page.locator('div#tooltip-container [class*="modal"]'),
                page.locator('div#tooltip-container [class*="dialog"]'),
            ]
        )

    async def _find_cover_editor_modal(self, page):
        return await self._find_first_visible_locator(
            [
                page.locator('div.dy-creator-content-modal:has-text("设置横封面"):has-text("设置竖封面")'),
                page.locator('div[role="dialog"]:has-text("设置横封面"):has-text("设置竖封面")'),
                page.locator('div#tooltip-container [class*="modal"]:has-text("上传封面"):has-text("完成")'),
                page.locator('div#tooltip-container [class*="dialog"]:has-text("上传封面"):has-text("完成")'),
                page.locator('div:has-text("设置竖封面"):has-text("上传封面"):has-text("完成")'),
            ]
        )

    async def _find_cover_entry(self, page):
        return await self._find_first_visible_locator(
            [
                page.get_by_text("选择封面", exact=True),
                page.get_by_role("button", name="选择封面"),
                page.locator('button:has-text("选择封面")'),
                page.locator('div:has-text("选择封面")'),
            ]
        )

    async def _find_cover_upload_input(self, page):
        editor_modal = await self._find_cover_editor_modal(page)
        search_roots = []
        if editor_modal is not None:
            search_roots.append(editor_modal)
        search_roots.append(page)

        selectors = [
            'input.semi-upload-hidden-input',
            'input[type="file"]',
        ]

        for root in search_roots:
            for selector in selectors:
                try:
                    locator = root.locator(selector)
                    count = await locator.count()
                except Exception:
                    continue
                if count <= 0:
                    continue
                return locator.nth(count - 1)
        return None

    async def _find_cover_confirm_button(self, page):
        return await self._find_first_visible_locator(
            [
                page.get_by_role("button", name="确定"),
                page.get_by_role("button", name="完成"),
                page.locator('div#tooltip-container button:visible:has-text("完成")'),
                page.locator('div.dy-creator-content-modal button:has-text("完成")'),
                page.locator('div[role="dialog"] button:has-text("完成")'),
                page.locator('button:has-text("应用")'),
            ]
        )

    async def _switch_vertical_cover_tab(self, page):
        editor_modal = await self._find_cover_editor_modal(page)
        search_roots = []
        if editor_modal is not None:
            search_roots.append(editor_modal)
        search_roots.append(page)

        for root in search_roots:
            tab = await self._find_first_visible_locator(
                [
                    root.get_by_role("button", name="设置竖封面"),
                    root.get_by_text("设置竖封面", exact=True),
                    root.locator('button:has-text("设置竖封面")'),
                    root.locator('[role="tab"]:has-text("设置竖封面")'),
                    root.locator('div:has-text("设置竖封面")'),
                ]
            )
            if tab is None:
                continue
            try:
                await tab.click(force=True)
                await page.wait_for_timeout(500)
                return True
            except Exception:
                continue
        return False

    async def _find_vertical_cover_prompt(self, page):
        return await self._find_first_visible_locator(
            [
                page.locator('div[role="dialog"]:has-text("设置竖封面获取更多流量")'),
                page.locator('div.dy-creator-content-modal:has-text("设置竖封面获取更多流量")'),
                page.locator('div#tooltip-container [class*="modal"]:has-text("设置竖封面获取更多流量")'),
                page.locator('div#tooltip-container [class*="dialog"]:has-text("设置竖封面获取更多流量")'),
                page.locator('div:has-text("设置竖封面获取更多流量"):has-text("暂不设置")'),
                page.locator('div[role="dialog"]:has-text("设置竖封面"):has-text("暂不设置")'),
                page.locator('div:has-text("设置竖封面"):has-text("暂不设置"):has-text("获取更多流量")'),
            ]
        )

    async def _dismiss_vertical_cover_prompt(self, page):
        try:
            prompt = await self._find_vertical_cover_prompt(page)
        except Exception:
            return False
        if prompt is None:
            return False

        dismiss_button = await self._find_first_visible_locator(
            [
                prompt.get_by_role("button", name="暂不设置"),
                prompt.locator('button:has-text("暂不设置")'),
                prompt.locator('[role="button"]:has-text("暂不设置")'),
                page.get_by_role("button", name="暂不设置"),
                page.get_by_text("暂不设置", exact=True),
            ]
        )
        close_button = None
        if dismiss_button is None:
            close_button = await self._find_first_visible_locator(
                [
                    prompt.locator('button[aria-label="关闭"]'),
                    prompt.locator('button[aria-label="close"]'),
                    prompt.locator('[aria-label="关闭"]'),
                    prompt.locator('[aria-label="close"]'),
                    prompt.locator('[class*="close"]:visible'),
                    prompt.locator('button:has(svg)'),
                    prompt.locator('svg[style*="cursor: pointer"]'),
                ]
            )

        try:
            if dismiss_button is not None:
                await dismiss_button.click(force=True)
                douyin_logger.info("  [-] 检测到“设置竖封面获取更多流量”弹窗，已选择暂不设置")
            elif close_button is not None:
                await close_button.click(force=True)
                douyin_logger.info("  [-] 检测到“设置竖封面获取更多流量”弹窗，已自动关闭")
            else:
                douyin_logger.warning("  [-] 检测到“设置竖封面获取更多流量”弹窗，但未找到可点击的关闭入口")
                return False
        except Exception as exc:
            douyin_logger.warning("  [-] 关闭“设置竖封面获取更多流量”弹窗失败: {}", exc)
            return False

        await page.wait_for_timeout(500)
        try:
            return await self._find_vertical_cover_prompt(page) is None
        except Exception:
            return True

    async def _wait_for_cover_editor_closed(self, page, timeout_ms=10000):
        deadline = asyncio.get_event_loop().time() + max(timeout_ms, 1000) / 1000
        while asyncio.get_event_loop().time() < deadline:
            await self._dismiss_vertical_cover_prompt(page)
            editor_modal = await self._find_cover_editor_modal(page)
            if editor_modal is None:
                return True
            await page.wait_for_timeout(400)
        return False

    async def _cover_warning_visible(self, page):
        for text in ["请设置封面后再发布", "请先设置封面", "请选择封面后再发布", "请选择封面"]:
            try:
                if await page.get_by_text(text).first.is_visible():
                    return True
            except Exception:
                continue
        return False

    async def _cover_requirement_satisfied(self, page):
        if await self._cover_warning_visible(page):
            return False
        cover_modal = await self._find_cover_modal(page)
        return cover_modal is None

    async def _open_cover_picker(self, page):
        cover_entry = await self._find_cover_entry(page)
        if cover_entry is None:
            return False
        await cover_entry.click(force=True)
        await page.wait_for_timeout(600)
        return True

    async def _select_recommend_cover(self, page):
        search_roots = []
        cover_modal = await self._find_cover_modal(page)
        if cover_modal is not None:
            search_roots.append(cover_modal)
        search_roots.append(page)

        for root in search_roots:
            recommend_cover = await self._find_recommend_cover(root)
            if recommend_cover is None:
                continue
            try:
                await recommend_cover.click(force=True)
                await page.wait_for_timeout(500)
                return True
            except Exception:
                continue
        return False

    async def _confirm_cover_selection(self, page):
        confirm_button = await self._find_cover_confirm_button(page)
        if confirm_button is None:
            return False
        await confirm_button.click(force=True)
        await page.wait_for_timeout(800)
        return True

    async def handle_auto_video_cover(self, page, max_attempts=4):
        """
        处理必须设置封面的情况，自动选择一个可用封面
        """
        try:
            for attempt in range(1, max(1, int(max_attempts)) + 1):
                if await self._cover_requirement_satisfied(page):
                    return True

                prompt_dismissed = await self._dismiss_vertical_cover_prompt(page)
                if prompt_dismissed:
                    await page.wait_for_timeout(400)
                    if await self._cover_requirement_satisfied(page):
                        douyin_logger.info("  [-] 已处理竖封面提示弹窗，沿用当前封面继续发布 attempt={}", attempt)
                        return True

                warning_visible = await self._cover_warning_visible(page)
                cover_modal = await self._find_cover_modal(page)
                if cover_modal is None:
                    if not warning_visible and attempt == 1:
                        return False
                    opened = await self._open_cover_picker(page)
                    if not opened:
                        douyin_logger.warning("  [-] 检测到封面缺失，但未找到“选择封面”入口")
                        await page.wait_for_timeout(400)
                        continue
                    douyin_logger.info("  [-] 检测到需要设置封面，正在打开封面选择器... attempt={}", attempt)

                douyin_logger.info("  [-] 正在自动选择推荐封面... attempt={}", attempt)
                selected = await self._select_recommend_cover(page)
                if not selected:
                    douyin_logger.warning("  [-] 已打开封面选择器，但未找到可点击的推荐封面 attempt={}", attempt)
                    await page.wait_for_timeout(500)
                    continue

                await self._confirm_cover_selection(page)
                await self._dismiss_vertical_cover_prompt(page)
                await page.wait_for_timeout(600)

                if await self._cover_requirement_satisfied(page):
                    douyin_logger.info("  [-] 已完成封面自动选择 attempt={}", attempt)
                    return True

                douyin_logger.warning("  [-] 封面选择后平台仍未确认，准备重试 attempt={}", attempt)

            douyin_logger.warning("  [-] 自动选择封面未通过平台校验，达到最大重试次数")
        except Exception as exc:
            douyin_logger.warning("  [-] 自动选择封面失败: {}", exc)
        return False

    async def set_thumbnail(self, page: Page, thumbnail_path: str):
        if thumbnail_path:
            douyin_logger.info('  [-] 正在设置视频封面...')
            opened = await self._open_cover_picker(page)
            if not opened:
                raise RuntimeError("未找到“选择封面”入口")

            editor_modal = await self._find_cover_editor_modal(page)
            if editor_modal is None:
                raise RuntimeError("未检测到封面编辑器")

            await self._dismiss_vertical_cover_prompt(page)
            await self._switch_vertical_cover_tab(page)
            await page.wait_for_timeout(800)

            upload_input = await self._find_cover_upload_input(page)
            if upload_input is None:
                raise RuntimeError("未找到封面上传输入框")
            await upload_input.set_input_files(thumbnail_path)
            await page.wait_for_timeout(1800)

            await self._dismiss_vertical_cover_prompt(page)
            confirmed = await self._confirm_cover_selection(page)
            if not confirmed:
                raise RuntimeError("未找到封面编辑器的完成按钮")

            await self._dismiss_vertical_cover_prompt(page)
            closed = await self._wait_for_cover_editor_closed(page)
            if not closed:
                raise RuntimeError("封面编辑器未正常关闭")

            douyin_logger.info('  [+] 视频封面设置完成！')
            

    async def set_location(self, page: Page, location: str = ""):
        if not location:
            return
        # todo supoort location later
        # await page.get_by_text('添加标签').locator("..").locator("..").locator("xpath=following-sibling::div").locator(
        #     "div.semi-select-single").nth(0).click()
        await page.locator('div.semi-select span:has-text("输入地理位置")').click()
        await page.keyboard.press("Backspace")
        await page.wait_for_timeout(2000)
        await page.keyboard.type(location)
        await page.wait_for_selector('div[role="listbox"] [role="option"]', timeout=5000)
        await page.locator('div[role="listbox"] [role="option"]').first.click()

    async def handle_product_dialog(self, page: Page, product_title: str):
        """处理商品编辑弹窗"""

        await page.wait_for_timeout(2000)
        await page.wait_for_selector('input[placeholder="请输入商品短标题"]', timeout=10000)
        short_title_input = page.locator('input[placeholder="请输入商品短标题"]')
        if not await short_title_input.count():
            douyin_logger.error("[-] 未找到商品短标题输入框")
            return False
        product_title = product_title[:10]
        await short_title_input.fill(product_title)
        # 等待一下让界面响应
        await page.wait_for_timeout(1000)

        finish_button = page.locator('button:has-text("完成编辑")')
        if 'disabled' not in await finish_button.get_attribute('class'):
            await finish_button.click()
            douyin_logger.debug("[+] 成功点击'完成编辑'按钮")
            
            # 等待对话框关闭
            await page.wait_for_selector('.semi-modal-content', state='hidden', timeout=5000)
            return True
        else:
            douyin_logger.error("[-] '完成编辑'按钮处于禁用状态，尝试直接关闭对话框")
            # 如果按钮禁用，尝试点击取消或关闭按钮
            cancel_button = page.locator('button:has-text("取消")')
            if await cancel_button.count():
                await cancel_button.click()
            else:
                # 点击右上角的关闭按钮
                close_button = page.locator('.semi-modal-close')
                await close_button.click()
            
            await page.wait_for_selector('.semi-modal-content', state='hidden', timeout=5000)
            return False
        
    async def set_product_link(self, page: Page, product_link: str, product_title: str):
        """设置商品链接功能"""
        await page.wait_for_timeout(2000)  # 等待2秒
        try:
            # 定位"添加标签"文本，然后向上导航到容器，再找到下拉框
            await page.wait_for_selector('text=添加标签', timeout=10000)
            dropdown = page.get_by_text('添加标签').locator("..").locator("..").locator("..").locator(".semi-select").first
            if not await dropdown.count():
                douyin_logger.error("[-] 未找到标签下拉框")
                return False
            douyin_logger.debug("[-] 找到标签下拉框，准备选择'购物车'")
            await dropdown.click()
            ## 等待下拉选项出现
            await page.wait_for_selector('[role="listbox"]', timeout=5000)
            ## 选择"购物车"选项
            await page.locator('[role="option"]:has-text("购物车")').click()
            douyin_logger.debug("[+] 成功选择'购物车'")
            
            # 输入商品链接
            ## 等待商品链接输入框出现
            await page.wait_for_selector('input[placeholder="粘贴商品链接"]', timeout=5000)
            # 输入
            input_field = page.locator('input[placeholder="粘贴商品链接"]')
            await input_field.fill(product_link)
            douyin_logger.debug(f"[+] 已输入商品链接: {product_link}")
            
            # 点击"添加链接"按钮
            add_button = page.locator('span:has-text("添加链接")')
            ## 检查按钮是否可用（没有disable类）
            button_class = await add_button.get_attribute('class')
            if 'disable' in button_class:
                douyin_logger.error("[-] '添加链接'按钮不可用")
                return False
            await add_button.click()
            douyin_logger.debug("[+] 成功点击'添加链接'按钮")
            ## 如果链接不可用
            await page.wait_for_timeout(2000)
            error_modal = page.locator('text=未搜索到对应商品')
            if await error_modal.count():
                confirm_button = page.locator('button:has-text("确定")')
                await confirm_button.click()
                # await page.wait_for_selector('.semi-modal-content', state='hidden', timeout=5000)
                douyin_logger.error("[-] 商品链接无效")
                return False

            # 填写商品短标题
            if not await self.handle_product_dialog(page, product_title):
                return False
            
            # 等待链接添加完成
            douyin_logger.debug("[+] 成功设置商品链接")
            return True
        except Exception as e:
            douyin_logger.error(f"[-] 设置商品链接时出错: {str(e)}")
            return False

    async def main(self):
        async with async_playwright() as playwright:
            await self.upload(playwright)
