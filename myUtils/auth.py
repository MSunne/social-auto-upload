import asyncio
import configparser
import os

from playwright.async_api import async_playwright
from xhs import XhsClient

from conf import BASE_DIR
from utils.base_social_media import set_init_script
from utils.browser_hook import get_browser_options
from utils.log import tencent_logger, kuaishou_logger, douyin_logger, xiaohongshu_logger
from pathlib import Path
from uploader.xhs_uploader.main import sign_local


async def cookie_auth_douyin(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        try:
            context = await browser.new_context(storage_state=account_file)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.douyin.com/creator-micro/content/upload")
            try:
                await page.wait_for_url("https://creator.douyin.com/creator-micro/content/upload", timeout=5000)
                try:
                    await page.get_by_text("扫码登录").wait_for(timeout=5000)
                    douyin_logger.error("[+] cookie 失效，需要扫码登录")
                    return False
                except:
                    douyin_logger.success("[+]  cookie 有效")
                    return True
            except:
                douyin_logger.error("[+] 等待5秒 cookie 失效")
                return False
        finally:
            await browser.close()


async def cookie_auth_tencent(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        try:
            context = await browser.new_context(storage_state=account_file)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://channels.weixin.qq.com/platform/post/create")
            try:
                await page.wait_for_selector('div.title-name:has-text("微信小店")', timeout=5000)
                tencent_logger.error("[+] 等待5秒 cookie 失效")
                return False
            except:
                tencent_logger.success("[+] cookie 有效")
                return True
        finally:
            await browser.close()


async def cookie_auth_ks(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        try:
            context = await browser.new_context(storage_state=account_file)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://cp.kuaishou.com/article/publish/video")
            try:
                await page.wait_for_selector("div.names div.container div.name:text('机构服务')", timeout=5000)
                kuaishou_logger.info("[+] 等待5秒 cookie 失效")
                return False
            except:
                kuaishou_logger.success("[+] cookie 有效")
                return True
        finally:
            await browser.close()


async def cookie_auth_xhs(account_file):
    async with async_playwright() as playwright:
        browser = await playwright.chromium.launch(**get_browser_options())
        try:
            context = await browser.new_context(storage_state=account_file)
            context = await set_init_script(context)
            page = await context.new_page()
            await page.goto("https://creator.xiaohongshu.com/creator-micro/content/upload")
            try:
                await page.wait_for_url("https://creator.xiaohongshu.com/creator-micro/content/upload", timeout=5000)
            except:
                xiaohongshu_logger.error("[+] 等待5秒 cookie 失效")
                return False
            if await page.get_by_text('手机号登录').count() or await page.get_by_text('扫码登录').count():
                xiaohongshu_logger.error("[+] 等待5秒 cookie 失效")
                return False
            xiaohongshu_logger.success("[+] cookie 有效")
            return True
        finally:
            await browser.close()


async def check_cookie(type, file_path):
    match type:
        # 小红书
        case 1:
            return await cookie_auth_xhs(Path(BASE_DIR / "cookiesFile" / file_path))
        # 视频号
        case 2:
            return await cookie_auth_tencent(Path(BASE_DIR / "cookiesFile" / file_path))
        # 抖音
        case 3:
            return await cookie_auth_douyin(Path(BASE_DIR / "cookiesFile" / file_path))
        # 快手
        case 4:
            return await cookie_auth_ks(Path(BASE_DIR / "cookiesFile" / file_path))
        case _:
            return False

# a = asyncio.run(check_cookie(1,"3a6cfdc0-3d51-11f0-8507-44e51723d63c.json"))
# print(a)
