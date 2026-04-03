import os
import subprocess
import sys
import threading
from pathlib import Path

from conf import LOCAL_CHROME_HEADLESS, LOCAL_CHROME_PATH
from utils.log import get_logger


POSIX_BROWSER_PATHS = [
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/snap/bin/chromium",
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
]

WINDOWS_BROWSER_ENV_CANDIDATES = (
    ("LOCALAPPDATA", "Google/Chrome/Application/chrome.exe"),
    ("PROGRAMFILES", "Google/Chrome/Application/chrome.exe"),
    ("PROGRAMFILES(X86)", "Google/Chrome/Application/chrome.exe"),
    ("LOCALAPPDATA", "Chromium/Application/chrome.exe"),
    ("PROGRAMFILES", "Chromium/Application/chrome.exe"),
    ("PROGRAMFILES(X86)", "Chromium/Application/chrome.exe"),
    ("PROGRAMFILES", "Microsoft/Edge/Application/msedge.exe"),
    ("PROGRAMFILES(X86)", "Microsoft/Edge/Application/msedge.exe"),
)

BASE_PLAYWRIGHT_BROWSER_DIRS = [
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
LINUX_GUI_ENV_KEYS = (
    "DISPLAY",
    "WAYLAND_DISPLAY",
    "XAUTHORITY",
    "DBUS_SESSION_BUS_ADDRESS",
    "XDG_RUNTIME_DIR",
    "XDG_SESSION_TYPE",
)


def _resolve_system_browser_executable_path():
    for candidate in _common_browser_path_candidates():
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

    candidate_roots.extend(_default_playwright_browser_dirs())

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


def _is_linux():
    return sys.platform.startswith("linux")


def _is_windows():
    return sys.platform.startswith("win")


def _common_browser_path_candidates():
    candidates = []
    if LOCAL_CHROME_PATH:
        candidates.append(LOCAL_CHROME_PATH)

    if _is_windows():
        for env_name, relative_path in WINDOWS_BROWSER_ENV_CANDIDATES:
            base_dir = str(os.getenv(env_name, "")).strip()
            if not base_dir:
                continue
            candidates.append(str(Path(base_dir) / relative_path))

    candidates.extend(POSIX_BROWSER_PATHS)

    seen = set()
    normalized = []
    for candidate in candidates:
        value = str(candidate or "").strip()
        if not value or value in seen:
            continue
        seen.add(value)
        normalized.append(value)
    return normalized


def _default_playwright_browser_dirs():
    roots = list(BASE_PLAYWRIGHT_BROWSER_DIRS)
    if _is_windows():
        local_appdata = str(os.getenv("LOCALAPPDATA", "")).strip()
        if local_appdata:
            roots.append(Path(local_appdata) / "ms-playwright")
    return roots


def _normalize_gui_env_value(value):
    text = str(value or "").strip()
    return text or None


def _extract_gui_env(source):
    env = {}
    for key in LINUX_GUI_ENV_KEYS:
        value = _normalize_gui_env_value(source.get(key) if source else None)
        if value:
            env[key] = value
    return env


def _finalize_linux_gui_env(env):
    normalized = dict(env or {})
    home = Path.home()
    xauthority_path = home / ".Xauthority"
    runtime_dir = Path(f"/run/user/{os.getuid()}")

    if not normalized.get("XAUTHORITY") and xauthority_path.exists():
        normalized["XAUTHORITY"] = str(xauthority_path)

    if not normalized.get("XDG_RUNTIME_DIR") and runtime_dir.exists():
        normalized["XDG_RUNTIME_DIR"] = str(runtime_dir)

    if not normalized.get("DBUS_SESSION_BUS_ADDRESS"):
        session_bus_path = runtime_dir / "bus"
        if session_bus_path.exists():
            normalized["DBUS_SESSION_BUS_ADDRESS"] = f"unix:path={session_bus_path}"

    if not normalized.get("DISPLAY"):
        for display in (":0", ":1"):
            if Path(f"/tmp/.X11-unix/X{display.lstrip(':')}").exists():
                normalized["DISPLAY"] = display
                break

    return normalized


def _read_linux_process_gui_env(proc_dir):
    try:
        if proc_dir.stat().st_uid != os.getuid():
            return {}
    except OSError:
        return {}

    environ_path = proc_dir / "environ"
    try:
        raw = environ_path.read_bytes()
    except OSError:
        return {}

    values = {}
    for entry in raw.split(b"\0"):
        if not entry or b"=" not in entry:
            continue
        key, value = entry.split(b"=", 1)
        try:
            text_key = key.decode("utf-8", errors="ignore")
            text_value = value.decode("utf-8", errors="ignore")
        except Exception:
            continue
        if text_key in LINUX_GUI_ENV_KEYS and _normalize_gui_env_value(text_value):
            values[text_key] = text_value.strip()
    return values


def resolve_linux_gui_environment():
    if not _is_linux():
        return {}, None

    current_env = _finalize_linux_gui_env(_extract_gui_env(os.environ))
    if current_env.get("DISPLAY") or current_env.get("WAYLAND_DISPLAY"):
        return current_env, "current_env"

    best_env = {}
    best_score = -1
    proc_root = Path("/proc")
    if proc_root.exists():
        for proc_dir in proc_root.iterdir():
            if not proc_dir.name.isdigit():
                continue
            candidate = _finalize_linux_gui_env(_read_linux_process_gui_env(proc_dir))
            if not (candidate.get("DISPLAY") or candidate.get("WAYLAND_DISPLAY")):
                continue
            score = len(candidate)
            if candidate.get("DISPLAY"):
                score += 5
            if candidate.get("XAUTHORITY"):
                score += 2
            if candidate.get("DBUS_SESSION_BUS_ADDRESS"):
                score += 1
            if score > best_score:
                best_score = score
                best_env = candidate

    if best_env:
        return best_env, "process_env"

    fallback_env = _finalize_linux_gui_env({})
    if fallback_env.get("DISPLAY") or fallback_env.get("WAYLAND_DISPLAY"):
        return fallback_env, "filesystem_fallback"

    return {}, None


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
    launch_env = {}
    launch_env_source = None

    if _is_linux() and not actual_headless:
        launch_env, launch_env_source = resolve_linux_gui_environment()

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
        if _is_linux() and not (launch_env.get("DISPLAY") or launch_env.get("WAYLAND_DISPLAY")):
            available = False
            issues.append(
                "Headed browser launch on Linux requires a desktop session, but no DISPLAY or WAYLAND_DISPLAY "
                "could be detected. Start SAU from the logged-in desktop session or configure a stable GUI environment."
            )

    return {
        "headless": actual_headless,
        "available": available,
        "source": source,
        "systemBrowserPath": system_browser_path,
        "playwrightBrowserDir": bundled_browser_dir,
        "checkedCandidates": _common_browser_path_candidates(),
        "launchEnvSource": launch_env_source,
        "launchEnvKeys": sorted(launch_env.keys()),
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

    if _is_linux() and not actual_headless:
        launch_env, launch_env_source = resolve_linux_gui_environment()
        if not (launch_env.get("DISPLAY") or launch_env.get("WAYLAND_DISPLAY")):
            raise RuntimeError(
                "Headed browser launch on Linux requires a desktop session, but no DISPLAY or WAYLAND_DISPLAY "
                "could be detected. Start SAU from the logged-in desktop session or configure a stable GUI environment."
            )
        merged_env = os.environ.copy()
        merged_env.update(launch_env)
        options["env"] = merged_env
        browser_logger.info(
            "resolved Linux GUI launch environment source={} display={} wayland={} xauthority_present={}",
            launch_env_source,
            launch_env.get("DISPLAY"),
            launch_env.get("WAYLAND_DISPLAY"),
            bool(launch_env.get("XAUTHORITY")),
        )

    return options
