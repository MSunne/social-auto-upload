import unittest
from unittest import mock

from utils import browser_hook


class BrowserHookTests(unittest.TestCase):
    def test_get_browser_options_uses_existing_system_browser_without_installing(self):
        with mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "install_playwright_browser",
        ) as install_mock:
            options = browser_hook.get_browser_options(headless=False)

        self.assertEqual(
            options["executable_path"],
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        )
        install_mock.assert_not_called()

    def test_get_browser_options_auto_installs_bundled_browser_when_missing(self):
        with mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            side_effect=[
                None,
                None,
                None,
                "/tmp/ms-playwright/chromium/chrome-mac/Chromium.app/Contents/MacOS/Chromium",
            ],
        ), mock.patch.object(
            browser_hook,
            "install_playwright_browser",
        ) as install_mock:
            options = browser_hook.get_browser_options(headless=False)

        self.assertEqual(
            options["executable_path"],
            "/tmp/ms-playwright/chromium/chrome-mac/Chromium.app/Contents/MacOS/Chromium",
        )
        install_mock.assert_called_once_with(browser_name="chromium")

    def test_get_browser_options_raises_clear_error_when_install_does_not_produce_browser(self):
        with mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "install_playwright_browser",
        ) as install_mock:
            with self.assertRaises(RuntimeError) as exc_info:
                browser_hook.get_browser_options(headless=False)

        self.assertIn("no Chromium executable", str(exc_info.exception))
        install_mock.assert_called_once_with(browser_name="chromium")


if __name__ == "__main__":
    unittest.main()
