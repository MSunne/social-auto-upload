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


def resolve_browser_executable_path():
    for candidate in COMMON_BROWSER_PATHS:
        if not candidate:
            continue
        path = Path(candidate).expanduser()
        if path.exists():
            return str(path)
    return None


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

    executable_path = resolve_browser_executable_path()
    if executable_path:
        options["executable_path"] = executable_path

    return options
