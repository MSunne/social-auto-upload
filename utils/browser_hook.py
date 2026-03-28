import os
import subprocess
import sys
import threading
from pathlib import Path

from conf import LOCAL_CHROME_HEADLESS, LOCAL_CHROME_PATH
from utils.log import get_logger


COMMON_BROWSER_PATHS = [
    LOCAL_CHROME_PATH,
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/snap/bin/chromium",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
]

COMMON_PLAYWRIGHT_BROWSER_DIRS = [
    Path("/opt/playwright"),
    Path.home() / ".cache" / "ms-playwright",
]

PLAYWRIGHT_EXECUTABLE_RELATIVE_PATHS = [
    Path("chrome-linux/chrome"),
    Path("chrome-mac/Chromium.app/Contents/MacOS/Chromium"),
    Path("chrome-win/chrome.exe"),
    Path("chrome-headless-shell-linux/headless_shell"),
    Path("chrome-headless-shell-mac/headless_shell"),
    Path("chrome-headless-shell-win/headless_shell.exe"),
]

_BROWSER_INSTALL_LOCK = threading.Lock()
browser_logger = get_logger("browser")


def _resolve_system_browser_executable_path():
    for candidate in COMMON_BROWSER_PATHS:
        if not candidate:
            continue
        path = Path(candidate).expanduser()
        if path.exists():
            return str(path)
    return None


def _playwright_browser_roots():
    env_path = str(os.getenv("PLAYWRIGHT_BROWSERS_PATH", "")).strip()
    candidate_roots = []

    if env_path == "0":
        candidate_roots.append(_resolve_playwright_package_root() / ".local-browsers")
    elif env_path:
        candidate_roots.append(Path(env_path).expanduser())

    candidate_roots.extend(COMMON_PLAYWRIGHT_BROWSER_DIRS)

    seen = set()
    roots = []
    for root in candidate_roots:
        resolved_root = root.resolve()
        key = str(resolved_root)
        if key in seen:
            continue
        seen.add(key)
        roots.append(resolved_root)
    return roots


def _resolve_playwright_package_root():
    import playwright

    return Path(playwright.__file__).resolve().parent / "driver" / "package"


def resolve_playwright_browser_executable_path():
    for root in _playwright_browser_roots():
        if not root.exists() or not root.is_dir():
            continue
        for child in sorted(root.iterdir()):
            if not child.is_dir():
                continue
            name = child.name.lower()
            if not (
                name.startswith("chromium")
                or name.startswith("chrome-linux")
                or name.startswith("chrome-headless-shell")
            ):
                continue
            for relative_path in PLAYWRIGHT_EXECUTABLE_RELATIVE_PATHS:
                executable_path = child / relative_path
                if executable_path.exists():
                    return str(executable_path)
    return None


def install_playwright_browser(browser_name="chromium"):
    command = [sys.executable, "-m", "playwright", "install", browser_name]
    browser_logger.info(
        "No usable browser runtime found. Installing Playwright browser browser_name={} command={}",
        browser_name,
        " ".join(command),
    )
    result = subprocess.run(
        command,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        stderr = (result.stderr or "").strip()
        stdout = (result.stdout or "").strip()
        details = stderr or stdout or "unknown error"
        browser_logger.error(
            "Playwright browser installation failed browser_name={} returncode={} details={}",
            browser_name,
            result.returncode,
            details,
        )
        raise RuntimeError(f"Playwright browser installation failed: {details}")
    browser_logger.info("Playwright browser installation completed browser_name={}", browser_name)


def _ensure_playwright_browser_executable_path(browser_name="chromium"):
    executable_path = resolve_playwright_browser_executable_path()
    if executable_path:
        return executable_path

    with _BROWSER_INSTALL_LOCK:
        executable_path = resolve_playwright_browser_executable_path()
        if executable_path:
            return executable_path
        install_playwright_browser(browser_name=browser_name)
        executable_path = resolve_playwright_browser_executable_path()
        if executable_path:
            return executable_path

    raise RuntimeError(
        "Playwright browser was installed, but no Chromium executable was found in the installation path."
    )


def resolve_browser_executable_path(*, headless=False):
    bundled_browser_path = resolve_playwright_browser_executable_path()
    if bundled_browser_path:
        return bundled_browser_path

    if headless:
        return None

    return _resolve_system_browser_executable_path()


def resolve_playwright_browser_dir():
    for root in _playwright_browser_roots():
        if not root.exists() or not root.is_dir():
            continue
        for child in root.iterdir():
            if not child.is_dir():
                continue
            name = child.name.lower()
            if (
                name.startswith("chromium")
                or name.startswith("chrome-linux")
                or name.startswith("chrome-headless-shell")
            ):
                return str(child)
    return None


def describe_browser_runtime(*, headless=None):
    actual_headless = LOCAL_CHROME_HEADLESS if headless is None else bool(headless)
    system_browser_path = _resolve_system_browser_executable_path()
    bundled_browser_dir = resolve_playwright_browser_dir()
    issues = []

    if actual_headless:
        available = bool(bundled_browser_dir)
        source = "playwright_bundled" if bundled_browser_dir else None
        if not bundled_browser_dir:
            issues.append(
                "Headless mode currently depends on Playwright bundled Chromium. "
                "Run `playwright install chromium` or disable LOCAL_CHROME_HEADLESS."
            )
        elif not system_browser_path:
            issues.append(
                "System browser was not found, but Playwright bundled Chromium is available."
            )
    else:
        if bundled_browser_dir:
            available = True
            source = "playwright_bundled"
            if not system_browser_path:
                issues.append(
                    "System browser was not found. OmniBull will fall back to Playwright bundled Chromium."
                )
        elif system_browser_path:
            available = True
            source = "system_browser"
        else:
            available = False
            source = None
            issues.append(
                "No usable Chrome/Chromium runtime was found. Install a system Chrome/Chromium browser "
                "or run `playwright install chromium`."
            )

    return {
        "headless": actual_headless,
        "available": available,
        "source": source,
        "systemBrowserPath": system_browser_path,
        "playwrightBrowserDir": bundled_browser_dir,
        "checkedCandidates": [path for path in COMMON_BROWSER_PATHS if path],
        "issues": issues,
    }


def get_browser_options(headless=None, extra_args=None):
    actual_headless = LOCAL_CHROME_HEADLESS if headless is None else headless
    extra_args = extra_args or []
    has_custom_lang = any(str(arg).startswith("--lang=") for arg in extra_args)
    args = [
        "--disable-blink-features=AutomationControlled",
        "--disable-infobars",
        "--disable-dev-shm-usage",
        "--no-sandbox",
        "--window-size=1600,1200",
    ]

    if not has_custom_lang:
        args.append("--lang=zh-CN")

    if not actual_headless:
        args.append("--start-maximized")

    args.extend(extra_args)

    # Preserve argument order while removing duplicates.
    deduped_args = list(dict.fromkeys(args))
    options = {
        "headless": actual_headless,
        "args": deduped_args,
    }

    executable_path = resolve_browser_executable_path(headless=actual_headless)
    if not executable_path:
        executable_path = _ensure_playwright_browser_executable_path("chromium")
    if executable_path:
        options["executable_path"] = executable_path

    return options
