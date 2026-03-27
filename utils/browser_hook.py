import os
from pathlib import Path

from conf import LOCAL_CHROME_HEADLESS, LOCAL_CHROME_PATH


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


def resolve_browser_executable_path(*, headless=False):
    if headless:
        # Headless validation works better with Playwright's bundled Chromium.
        # Avoid forcing the local Chrome app so macOS does not briefly bounce the
        # dock icon for background cookie checks.
        return None
    for candidate in COMMON_BROWSER_PATHS:
        if not candidate:
            continue
        path = Path(candidate).expanduser()
        if path.exists():
            return str(path)
    return None


def resolve_playwright_browser_dir():
    env_path = str(os.getenv("PLAYWRIGHT_BROWSERS_PATH", "")).strip()
    candidate_roots = []
    if env_path and env_path != "0":
        candidate_roots.append(Path(env_path).expanduser())

    candidate_roots.extend(COMMON_PLAYWRIGHT_BROWSER_DIRS)

    seen = set()
    for root in candidate_roots:
        resolved_root = root.resolve()
        key = str(resolved_root)
        if key in seen:
            continue
        seen.add(key)
        if not resolved_root.exists() or not resolved_root.is_dir():
            continue
        for child in resolved_root.iterdir():
            if not child.is_dir():
                continue
            name = child.name.lower()
            if name.startswith("chromium") or name.startswith("chrome-linux"):
                return str(child)
    return None


def describe_browser_runtime(*, headless=None):
    actual_headless = LOCAL_CHROME_HEADLESS if headless is None else bool(headless)
    system_browser_path = resolve_browser_executable_path(headless=False)
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
        if system_browser_path:
            available = True
            source = "system_browser"
        elif bundled_browser_dir:
            available = True
            source = "playwright_bundled"
            issues.append(
                "System browser was not found. OmniBull will fall back to Playwright bundled Chromium."
            )
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
    if executable_path:
        options["executable_path"] = executable_path

    return options
