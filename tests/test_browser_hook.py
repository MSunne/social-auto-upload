import unittest
from unittest import mock

from utils import browser_hook


class BrowserHookTests(unittest.TestCase):
    def test_common_browser_path_candidates_include_windows_defaults(self):
        with mock.patch.object(browser_hook, "_is_windows", return_value=True), mock.patch.dict(
            "os.environ",
            {
                "LOCALAPPDATA": r"C:\Users\sun\AppData\Local",
                "PROGRAMFILES": r"C:\Program Files",
                "PROGRAMFILES(X86)": r"C:\Program Files (x86)",
            },
            clear=False,
        ):
            candidates = browser_hook._common_browser_path_candidates()

        self.assertIn(r"C:\Users\sun\AppData\Local/Google/Chrome/Application/chrome.exe", candidates)
        self.assertIn(r"C:\Program Files/Chromium/Application/chrome.exe", candidates)
        self.assertIn(r"C:\Program Files (x86)/Microsoft/Edge/Application/msedge.exe", candidates)

    def test_default_playwright_browser_dirs_include_windows_localappdata(self):
        with mock.patch.object(browser_hook, "_is_windows", return_value=True), mock.patch.dict(
            "os.environ",
            {"LOCALAPPDATA": r"C:\Users\sun\AppData\Local"},
            clear=False,
        ):
            roots = browser_hook._default_playwright_browser_dirs()

        self.assertIn(browser_hook.Path(r"C:\Users\sun\AppData\Local") / "ms-playwright", roots)

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
        self.assertNotIn("--window-size=1600,1200", options["args"])
        self.assertNotIn("--start-maximized", options["args"])
        install_mock.assert_not_called()

    def test_resolve_browser_executable_path_prefers_system_browser_for_headed_launch(self):
        with mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value="/usr/bin/google-chrome",
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            return_value="/opt/playwright/chromium/chrome-linux/chrome",
        ):
            path = browser_hook.resolve_browser_executable_path(headless=False)

        self.assertEqual(path, "/usr/bin/google-chrome")

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

    def test_get_browser_options_injects_linux_gui_environment_for_headed_launch(self):
        launch_env = {
            "DISPLAY": ":0",
            "XAUTHORITY": "/home/sun/.Xauthority",
        }
        with mock.patch.object(
            browser_hook,
            "_is_linux",
            return_value=True,
        ), mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value="/usr/bin/google-chrome",
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "resolve_linux_gui_environment",
            return_value=(launch_env, "process_env"),
        ):
            options = browser_hook.get_browser_options(headless=False)

        self.assertEqual(options["executable_path"], "/usr/bin/google-chrome")
        self.assertIn("env", options)
        self.assertEqual(options["env"]["DISPLAY"], ":0")
        self.assertEqual(options["env"]["XAUTHORITY"], "/home/sun/.Xauthority")

    def test_get_browser_options_raises_clear_error_when_linux_gui_is_missing(self):
        with mock.patch.object(
            browser_hook,
            "_is_linux",
            return_value=True,
        ), mock.patch.object(
            browser_hook,
            "_resolve_system_browser_executable_path",
            return_value="/usr/bin/google-chrome",
        ), mock.patch.object(
            browser_hook,
            "resolve_playwright_browser_executable_path",
            return_value=None,
        ), mock.patch.object(
            browser_hook,
            "resolve_linux_gui_environment",
            return_value=({}, None),
        ):
            with self.assertRaises(RuntimeError) as exc_info:
                browser_hook.get_browser_options(headless=False)

        self.assertIn("Headed browser launch on Linux requires a desktop session", str(exc_info.exception))


if __name__ == "__main__":
    unittest.main()
