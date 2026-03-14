# -*- coding: utf-8 -*-
from datetime import datetime

from playwright.async_api import Playwright, async_playwright
import os
import asyncio

from conf import LOCAL_CHROME_HEADLESS
from utils.base_social_media import set_init_script
from utils.browser_hook import get_browser_options
from utils.creator_popup import dismiss_platform_popups
from utils.files_times import get_absolute_path
from utils.log import kuaishou_logger
from utils.publish_verification import PublishManualVerificationRequired, ensure_no_publish_verification


async def cookie_auth(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        context = await browser.new_context(storage_state=account_file)
        context = await set_init_script(context)
        # 创建一个新的页面
        page = await context.new_page()
        # 访问指定的 URL
        await page.goto("https://cp.kuaishou.com/article/publish/video")
        try:
            await page.wait_for_selector("div.names div.container div.name:text('机构服务')", timeout=5000)  # 等待5秒

            kuaishou_logger.info("[+] 等待5秒 cookie 失效")
            return False
        except:
            kuaishou_logger.success("[+] cookie 有效")
            return True


async def ks_setup(account_file, handle=False):
    account_file = get_absolute_path(account_file, "ks_uploader")
    if not os.path.exists(account_file) or not await cookie_auth(account_file):
        if not handle:
            return False
        kuaishou_logger.info('[+] cookie文件不存在或已失效，即将自动打开浏览器，请扫码登录，登陆后会自动生成cookie文件')
        await get_ks_cookie(account_file)
    return True


async def get_ks_cookie(account_file):
    async with async_playwright() as playwright:
        options = get_browser_options(extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://cp.kuaishou.com")
        await page.pause()
        # 点击调试器的继续，保存cookie
        await context.storage_state(path=account_file)


class KSVideo(object):
    def __init__(self, title, file_path, tags, publish_date: datetime, account_file):
        self.title = title  # 视频标题
        self.file_path = file_path
        self.tags = tags
        self.publish_date = publish_date
        self.account_file = account_file
        self.date_format = '%Y-%m-%d %H:%M'
        self.headless = LOCAL_CHROME_HEADLESS

    async def handle_upload_error(self, page):
        kuaishou_logger.error("视频出错了，重新上传中")
        await page.locator('div.progress-div [class^="upload-btn-input"]').set_input_files(self.file_path)

    async def upload(self, playwright: Playwright) -> None:
        # 使用 Chromium 浏览器启动一个浏览器实例
        browser = await playwright.chromium.launch(**get_browser_options(headless=self.headless))
        context = await browser.new_context(storage_state=f"{self.account_file}")
        context = await set_init_script(context)
        try:
            page = await context.new_page()
            await page.goto("https://cp.kuaishou.com/article/publish/video")
            kuaishou_logger.info('正在上传-------{}.mp4'.format(self.title))
            kuaishou_logger.info('正在打开主页...')
            await page.wait_for_url("https://cp.kuaishou.com/article/publish/video")
            await dismiss_platform_popups(page, "kuaishou")
            await ensure_no_publish_verification(page, "快手")
            upload_button = page.locator("button[class^='_upload-btn']")
            await upload_button.wait_for(state='visible')

            async with page.expect_file_chooser() as fc_info:
                await upload_button.click()
            file_chooser = await fc_info.value
            await file_chooser.set_files(self.file_path)

            await asyncio.sleep(2)
            await dismiss_platform_popups(page, "kuaishou")
            await ensure_no_publish_verification(page, "快手")
            await asyncio.sleep(1)
            await dismiss_platform_popups(page, "kuaishou")

            kuaishou_logger.info("正在填充标题和话题...")
            await page.get_by_text("描述").locator("xpath=following-sibling::div").click()
            kuaishou_logger.info("clear existing title")
            await page.keyboard.press("Backspace")
            await page.keyboard.press("Control+KeyA")
            await page.keyboard.press("Delete")
            kuaishou_logger.info("filling new  title")
            await page.keyboard.type(self.title)
            await page.keyboard.press("Enter")

            for index, tag in enumerate(self.tags[:3], start=1):
                kuaishou_logger.info("正在添加第%s个话题" % index)
                await page.keyboard.type(f"#{tag} ")
                await asyncio.sleep(2)

            max_retries = 60
            retry_count = 0

            while retry_count < max_retries:
                try:
                    await ensure_no_publish_verification(page, "快手")
                    number = await page.locator("text=上传中").count()

                    if number == 0:
                        kuaishou_logger.success("视频上传完毕")
                        break

                    if retry_count % 5 == 0:
                        kuaishou_logger.info("正在上传视频中...")
                    await asyncio.sleep(2)
                except PublishManualVerificationRequired:
                    raise
                except Exception as e:
                    kuaishou_logger.error(f"检查上传状态时发生错误: {e}")
                    await asyncio.sleep(2)
                retry_count += 1

            if retry_count == max_retries:
                kuaishou_logger.warning("超过最大重试次数，视频上传可能未完成。")

            if self.publish_date != 0:
                await self.set_schedule_time(page, self.publish_date)

            while True:
                try:
                    await dismiss_platform_popups(page, "kuaishou")
                    await ensure_no_publish_verification(page, "快手")
                    publish_button = page.get_by_text("发布", exact=True)
                    if await publish_button.count() > 0:
                        await publish_button.click()

                    await asyncio.sleep(1)
                    confirm_button = page.get_by_text("确认发布")
                    if await confirm_button.count() > 0:
                        await confirm_button.click()

                    await page.wait_for_url(
                        "https://cp.kuaishou.com/article/manage/video?status=2&from=publish",
                        timeout=5000,
                    )
                    kuaishou_logger.success("视频发布成功")
                    break
                except PublishManualVerificationRequired:
                    raise
                except Exception as e:
                    await dismiss_platform_popups(page, "kuaishou")
                    await ensure_no_publish_verification(page, "快手")
                    kuaishou_logger.info(f"视频正在发布中... 错误: {e}")
                    await page.screenshot(full_page=True)
                    await asyncio.sleep(1)

            await context.storage_state(path=self.account_file)
            kuaishou_logger.info('cookie更新完毕！')
            await asyncio.sleep(2)
        finally:
            await context.close()
            await browser.close()

    async def main(self):
        async with async_playwright() as playwright:
            await self.upload(playwright)

    async def set_schedule_time(self, page, publish_date):
        kuaishou_logger.info("click schedule")
        publish_date_hour = publish_date.strftime("%Y-%m-%d %H:%M:%S")
        await page.locator("label:text('发布时间')").locator('xpath=following-sibling::div').locator(
            '.ant-radio-input').nth(1).click()
        await asyncio.sleep(1)

        await page.locator('div.ant-picker-input input[placeholder="选择日期时间"]').click()
        await asyncio.sleep(1)

        await page.keyboard.press("Control+KeyA")
        await page.keyboard.type(str(publish_date_hour))
        await page.keyboard.press("Enter")
        await asyncio.sleep(1)
