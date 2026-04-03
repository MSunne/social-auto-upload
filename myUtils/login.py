import asyncio
import base64
import json
import os
import re
import sys
import uuid
from pathlib import Path
from queue import Empty

from playwright.async_api import async_playwright

from myUtils.auth import check_cookie_detail, validate_login_completion_detail
from utils.account_storage import upsert_login_account
from utils.base_social_media import set_init_script
from utils.browser_hook import get_browser_options
from utils.log import login_logger, log_throttled

VERIFICATION_TITLE_TEXTS = [
    "身份验证",
    "验证身份",
    "安全验证",
    "安全确认",
    "登录验证",
    "验证方式",
    "选择验证方式",
    "选择验证方式继续登录",
    "验证你的身份",
    "验证手机号",
    "手机验证",
    "短信验证",
    "验证码验证",
    "接收短信验证码",
    "接收短信验证",
    "发送短信验证",
    "发送短信验证码",
    "重新发送验证码",
    "重新获取验证码",
    "再次发送验证码",
    "请输入验证码",
]

VERIFICATION_OPTION_TEXTS = [
    "接收短信验证码",
    "接收短信验证",
    "发送短信验证",
    "发送短信验证码",
    "重新发送",
    "重新发送验证码",
    "重新获取验证码",
    "再次发送验证码",
    "重发验证码",
    "短信验证码",
    "验证登录密码",
    "登录密码",
    "密码验证",
    "获取验证码",
    "发送验证码",
    "短信验证",
    "手机验证",
    "验证身份",
    "下一步",
    "继续验证",
    "继续",
    "确认",
    "提交",
]

VERIFICATION_CODE_ACTION_TEXTS = [
    "接收短信验证码",
    "接收短信验证",
    "发送短信验证",
    "发送短信验证码",
    "获取验证码",
    "发送验证码",
    "重新发送",
    "重新发送验证码",
    "重新获取验证码",
    "再次发送验证码",
    "重发验证码",
]

VERIFICATION_PASSWORD_ACTION_TEXTS = [
    "验证登录密码",
    "登录密码",
    "密码验证",
]

VERIFICATION_SPECIFIC_CODE_ACTION_TEXTS = [
    "接收短信验证码",
    "接收短信验证",
    "重新发送",
    "重新发送验证码",
    "重新获取验证码",
    "再次发送验证码",
    "重发验证码",
]

WEAK_VERIFICATION_TITLES = {
    "需要额外验证",
}

WEAK_GENERIC_CODE_OPTIONS = {
    "获取验证码",
    "发送验证码",
    "短信验证",
    "手机验证",
}

VERIFICATION_SUBMIT_TEXTS = [
    "确认",
    "提交",
    "下一步",
    "继续",
    "继续验证",
    "登录",
    "验证",
    "完成",
]

QR_EXPIRED_TEXTS = [
    "二维码已过期",
    "二维码已失效",
    "二维码失效",
    "登录二维码已过期",
    "登录二维码已失效",
    "扫码已过期",
    "扫码已失效",
    "请点击刷新",
]

QR_SCANNED_TEXTS = [
    "已扫码，请在手机上确认登录",
    "已扫码，请在手机上确认",
    "扫码成功，请在手机上确认登录",
    "扫码成功，请在手机上确认",
    "请在手机上确认登录",
    "请在手机确认登录",
    "请在手机上点击确认",
    "需在手机上进行确认",
    "需在手机上进行确认登录",
    "请在手机上进行确认",
]

# Grace period (seconds) after QR image disappears before accepting weak verification challenges.
# During this window, only strong verification signals (e.g. "接收短信验证码") are accepted.
# This prevents the right-side login form elements from being misidentified as real verification.
QR_HIDDEN_GRACE_SECONDS = 12.0

QR_REFRESH_TEXTS = [
    "点击刷新",
    "刷新二维码",
    "重新获取二维码",
    "重新获取",
    "重新加载",
    "点击重试",
    "刷新",
]

INTERACTIVE_ACTION_SELECTOR = (
    "button, [role='button'], a[href], label, [tabindex], [onclick], "
    "input[type='button'], input[type='submit'], "
    "div[class*='btn'], span[class*='btn'], "
    "div[class*='button'], span[class*='button'], "
    "div[class*='submit'], span[class*='submit'], "
    "div[class*='confirm'], span[class*='confirm'], "
    "div[class*='verify'], span[class*='verify']"
)
INTERACTIVE_ACTION_CLOSEST_SELECTOR = (
    "button, [role='button'], a[href], label, [tabindex], [onclick], "
    "input[type='button'], input[type='submit'], "
    "[class*='btn'], [class*='button'], [class*='submit'], [class*='confirm'], [class*='verify']"
)

QR_SNAPSHOT_MIN_INTERVAL_SECONDS = 2.0
VERIFICATION_ROOT_MARKER_ATTR = "data-omnibull-verification-root"
TRANSIENT_LOGIN_ERROR_HINTS = (
    "execution context was destroyed",
    "most likely because of a navigation",
    "target page, context or browser has been closed",
    "frame was detached",
    "navigation interrupted",
    "cannot find context with specified id",
)
VERIFICATION_INPUT_HINT_KEYWORDS = [
    "验证码",
    "短信",
    "密码",
    "手机",
    "验证",
]
EDITABLE_INPUT_SELECTOR = (
    "input, textarea, [contenteditable]:not([contenteditable='false']), "
    "[role='textbox'], [role='searchbox'], [role='combobox'], [aria-multiline='true']"
)
FOCUSED_EDITABLE_SELECTORS = [
    "input:focus",
    "textarea:focus",
    "[contenteditable]:focus",
    "[role='textbox']:focus",
    "[role='searchbox']:focus",
    "[role='combobox']:focus",
    "[aria-multiline='true']:focus",
]
REMOTE_ACTION_RETRY_WINDOW_SECONDS = 120.0
REMOTE_ACTION_MAX_RETRIES = 240
LOGIN_DEBUG_LOG_DIR = Path("logs")


class LoginCancelled(Exception):
    pass


class LoginPersistFailed(Exception):
    pass


def is_transient_login_page_error(exc):
    message = str(exc or "").strip().lower()
    if not message:
        return False
    return any(hint in message for hint in TRANSIENT_LOGIN_ERROR_HINTS)


def save_login_account(account_type, user_name, file_name, status=1, storage_state=None):
    """同平台同账号只保留一条记录，登录态以数据库为主存。"""
    return upsert_login_account(
        account_type,
        user_name,
        file_name=file_name,
        status=status,
        storage_state=storage_state,
    )


async def locator_to_data_url(locator):
    await locator.wait_for(state="visible", timeout=30000)
    image_bytes = await locator.screenshot(type="png")
    return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")


async def locator_to_data_url_if_visible(locator):
    if locator is None:
        return None
    try:
        count = await locator.count()
        if not count:
            return None
        candidate = locator.first
        if not await candidate.is_visible():
            return None
        image_bytes = await candidate.screenshot(type="png")
        return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")
    except Exception:
        return None


async def get_locator_visual_signature_if_visible(locator):
    if locator is None:
        return None
    try:
        count = await locator.count()
        if not count:
            return None
        candidate = locator.first
        if not await candidate.is_visible():
            return None
        return await candidate.evaluate(
            """
            (element) => {
                const tag = (element.tagName || "").toLowerCase();
                if (tag === "img") {
                    return `img:${element.currentSrc || element.src || ""}`;
                }
                if (tag === "canvas") {
                    try {
                        return `canvas:${element.width || 0}x${element.height || 0}:${element.toDataURL("image/png").slice(0, 256)}`;
                    } catch (error) {
                        return `canvas:${element.width || 0}x${element.height || 0}`;
                    }
                }
                const backgroundImage = window.getComputedStyle(element).backgroundImage || "";
                const text = (element.innerText || element.textContent || "").trim();
                return `${tag}:${backgroundImage}:${text}`.slice(0, 512);
            }
            """
        )
    except Exception:
        return None


async def is_locator_visible(locator):
    if locator is None:
        return False
    try:
        count = await locator.count()
        if not count:
            return False
        return await locator.first.is_visible()
    except Exception:
        return False


async def page_to_data_url(page, locator=None):
    if locator is not None:
        try:
            if await locator.count() and await locator.first.is_visible():
                image_bytes = await locator.first.screenshot(type="png")
                return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")
        except Exception:
            pass

    image_bytes = await page.screenshot(type="png")
    return "data:image/png;base64," + base64.b64encode(image_bytes).decode("utf-8")


async def find_first_visible_text(page, texts):
    for text in texts:
        locator = page.get_by_text(text, exact=True)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


async def find_first_visible_partial_text(page, texts):
    for text in texts:
        locator = page.get_by_text(text)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


async def find_first_visible_containing_text(target, texts):
    for text in texts:
        locator = target.get_by_text(text)
        count = await locator.count()
        for index in range(count):
            candidate = locator.nth(index)
            try:
                if await candidate.is_visible():
                    return text, candidate
            except Exception:
                continue
    return None, None


def normalize_action_text(text):
    return " ".join(str(text or "").split())


def canonicalize_verification_option_label(label):
    normalized = normalize_action_text(label)
    if not normalized:
        return ""

    resend_patterns = (
        r"^\d+\s*(?:s|秒)后重新发送(?:验证码)?$",
        r"^\d+\s*(?:s|秒)后重发验证码$",
        r"^\d+\s*(?:s|秒)后再次发送验证码$",
        r"^\d+\s*(?:s|秒)后重新获取验证码$",
    )
    if any(re.match(pattern, normalized, flags=re.IGNORECASE) for pattern in resend_patterns):
        return "重新发送验证码"

    if any(keyword in normalized for keyword in ("重新发送", "重发验证码", "再次发送验证码", "重新获取验证码")):
        if re.search(r"\d", normalized):
            return "重新发送验证码"

    return normalized


def score_action_label_match(label, search_text):
    normalized_label = normalize_action_text(label)
    normalized_search = normalize_action_text(search_text)
    if not normalized_label or not normalized_search:
        return -1
    if normalized_label == normalized_search:
        return 100 + min(len(normalized_search), 12)

    canonical_label = canonicalize_verification_option_label(normalized_label)
    if canonical_label == normalized_search:
        return 90 + min(len(normalized_search), 12)

    match_target = canonical_label if normalized_search in canonical_label else normalized_label
    if normalized_search not in match_target:
        return -1

    if len(normalized_search) <= 3:
        if len(match_target) > max(len(normalized_search) + 2, 4):
            return -1
        return 70 - max(0, len(match_target) - len(normalized_search))

    return 75 - max(0, len(match_target) - len(normalized_search))


async def get_verification_anchor(page):
    title, title_locator = await find_first_visible_text(page, VERIFICATION_TITLE_TEXTS)
    if title_locator is None:
        title, title_locator = await find_first_visible_partial_text(page, VERIFICATION_TITLE_TEXTS)
    option_texts = await collect_visible_option_texts(page)

    anchor_locator = title_locator
    if anchor_locator is None and option_texts:
        _, anchor_locator = await find_first_visible_partial_text(page, [option_texts[0]])

    if anchor_locator is None:
        page_inputs = await iter_visible_editable_inputs(page)
        if page_inputs:
            anchor_locator = page_inputs[0]

    return title, option_texts, anchor_locator


async def collect_visible_option_texts(page):
    visible_texts = []
    interactive_locator = page.locator(INTERACTIVE_ACTION_SELECTOR)
    try:
        interactive_count = await interactive_locator.count()
    except Exception:
        interactive_count = 0

    for index in range(min(interactive_count, 80)):
        candidate = interactive_locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
            raw_text = await candidate.evaluate(
                """
                (element) => {
                    const values = [
                        element.innerText,
                        element.textContent,
                        element.value,
                        element.getAttribute("aria-label"),
                        element.getAttribute("title"),
                    ];
                    return values.filter(Boolean).join("\\n");
                }
                """
            )
        except Exception:
            continue

        for line in str(raw_text or "").splitlines():
            label = " ".join(line.split())
            if not label or len(label) > 24:
                continue
            if any(keyword in label for keyword in VERIFICATION_OPTION_TEXTS):
                visible_texts.append(label)

    if visible_texts:
        return dedupe_verification_option_labels(visible_texts)

    for text in VERIFICATION_OPTION_TEXTS:
        for locator in (page.get_by_text(text, exact=True), page.get_by_text(text)):
            count = await locator.count()
            found_visible = False
            for index in range(count):
                candidate = locator.nth(index)
                try:
                    if await candidate.is_visible():
                        found_visible = True
                        break
                except Exception:
                    continue
            if found_visible:
                visible_texts.append(text)
                break
    return dedupe_verification_option_labels(visible_texts)


def dedupe_verification_option_labels(labels):
    unique_labels = []
    seen_canonical = set()
    for label in labels:
        normalized = normalize_action_text(label)
        canonical = canonicalize_verification_option_label(normalized)
        if not canonical or canonical in seen_canonical:
            continue
        seen_canonical.add(canonical)
        unique_labels.append(normalized)

    if len(unique_labels) <= 1:
        return unique_labels

    non_submit_labels = [
        label for label in unique_labels if label not in VERIFICATION_SUBMIT_TEXTS
    ]
    candidate_labels = non_submit_labels or unique_labels

    filtered_labels = []
    for label in candidate_labels:
        if any(label != other and label in other for other in candidate_labels):
            continue
        filtered_labels.append(label)

    return filtered_labels or candidate_labels


def refine_verification_option_labels(labels, title=None, input_hints=None):
    refined_labels = dedupe_verification_option_labels(labels)
    if len(refined_labels) <= 1:
        return refined_labels

    normalized_title = " ".join(str(title or "").split())
    normalized_hints = [" ".join(str(item or "").split()) for item in (input_hints or []) if str(item or "").strip()]

    has_code_context = (
        any("验证码" in hint for hint in normalized_hints)
        or any(keyword in normalized_title for keyword in VERIFICATION_CODE_ACTION_TEXTS)
    )

    if has_code_context:
        code_related = [
            label
            for label in refined_labels
            if any(keyword in label for keyword in VERIFICATION_CODE_ACTION_TEXTS)
        ]
        if code_related:
            refined_labels = code_related

        refined_labels = [
            label
            for label in refined_labels
            if not any(keyword in label for keyword in VERIFICATION_PASSWORD_ACTION_TEXTS)
        ] or refined_labels

        specific_code_labels = [
            label
            for label in refined_labels
            if any(keyword in label for keyword in VERIFICATION_SPECIFIC_CODE_ACTION_TEXTS)
        ]
        if specific_code_labels:
            refined_labels = [
                label
                for label in refined_labels
                if label not in {"获取验证码", "发送验证码", "短信验证", "手机验证"}
            ] or specific_code_labels

    return dedupe_verification_option_labels(refined_labels)


def is_strong_verification_challenge(challenge):
    payload = (challenge or {}).get("payload") or {}
    title = " ".join(str(payload.get("title") or "").split())
    options = [
        " ".join(str(item or "").split())
        for item in (payload.get("options") or [])
        if str(item or "").strip()
    ]
    input_hints = [
        " ".join(str(item or "").split())
        for item in (payload.get("inputHints") or [])
        if str(item or "").strip()
    ]
    supports_text_input = bool(payload.get("supportsTextInput"))

    if not title and not options and not input_hints:
        return False

    if title and title not in WEAK_VERIFICATION_TITLES:
        return True

    if any(
        keyword in option
        for option in options
        for keyword in (
            "接收短信",
            "重新发送",
            "重发验证码",
            "登录密码",
            "验证登录密码",
            "密码验证",
        )
    ):
        return True

    specific_options = [
        option for option in options if option and option not in WEAK_GENERIC_CODE_OPTIONS
    ]
    if specific_options:
        return True

    if supports_text_input and len(options) >= 2:
        return True

    return False


async def get_visible_verification_input_hints(page):
    hints = []
    locator = page.locator(EDITABLE_INPUT_SELECTOR)
    count = await locator.count()
    for index in range(min(count, 8)):
        candidate = locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
            meta_payload = await get_editable_meta(candidate)
            if meta_payload is None:
                continue
            if meta_payload.get("readonly") or meta_payload.get("disabled"):
                continue
            input_type = str(meta_payload.get("type") or "text").lower()
            if input_type in {"hidden", "file", "checkbox", "radio"}:
                continue
            placeholder = str(meta_payload.get("placeholder") or "").strip()
            if placeholder and any(keyword in placeholder for keyword in VERIFICATION_INPUT_HINT_KEYWORDS):
                hints.append(placeholder)
                continue
            label_text = str(meta_payload.get("ariaLabel") or "").strip()
            if label_text and any(keyword in label_text for keyword in VERIFICATION_INPUT_HINT_KEYWORDS):
                hints.append(label_text)
                continue
            if input_type in {"password", "tel", "number"}:
                hints.append("请输入验证码或密码")
                continue
            if "one-time-code" in str(meta_payload.get("autocomplete") or ""):
                hints.append("请输入验证码")
                continue
            if (
                str(meta_payload.get("inputmode") or "") in {"numeric", "decimal", "tel"}
                and isinstance(meta_payload.get("maxlength"), int)
                and 4 <= int(meta_payload.get("maxlength")) <= 8
            ):
                hints.append("请输入验证码")
                continue
        except Exception:
            continue
    return list(dict.fromkeys(hints))


async def get_visible_verification_status_texts(page, exclude_texts=None):
    normalized_excludes = {
        normalize_action_text(text)
        for text in (exclude_texts or [])
        if normalize_action_text(text)
    }

    try:
        editable_inputs = await iter_visible_editable_inputs(page)
    except Exception:
        editable_inputs = []

    for candidate in editable_inputs:
        try:
            current_value = normalize_action_text(await get_editable_value(candidate))
        except Exception:
            current_value = ""
        if current_value:
            normalized_excludes.add(current_value)

    try:
        lines = await page.evaluate(
            """
            (element, payload) => {
                const excluded = new Set(payload.excluded || []);
                const nodes = [element, ...Array.from(element.querySelectorAll('*'))];
                const results = [];
                const seen = new Set();

                for (const node of nodes) {
                    const style = window.getComputedStyle(node);
                    const rect = node.getBoundingClientRect();
                    if (
                        style.visibility === 'hidden'
                        || style.display === 'none'
                        || style.opacity === '0'
                        || rect.width < 2
                        || rect.height < 2
                    ) {
                        continue;
                    }

                    const rawText = (node.innerText || node.textContent || '');
                    const splitLines = rawText
                        .split(/\\n+/)
                        .map((line) => line.replace(/\\s+/g, ' ').trim())
                        .filter(Boolean);

                    for (const line of splitLines) {
                        if (!line || line.length < 2 || line.length > 32) {
                            continue;
                        }
                        if (/^\\d{4,8}$/.test(line)) {
                            continue;
                        }
                        if (excluded.has(line) || seen.has(line)) {
                            continue;
                        }
                        seen.add(line);
                        results.push(line);
                    }
                }

                return results.slice(0, 8);
            }
            """,
            {"excluded": list(normalized_excludes)},
        )
    except Exception:
        return []

    filtered = []
    for line in lines or []:
        normalized = normalize_action_text(line)
        if not normalized or normalized in normalized_excludes:
            continue
        filtered.append(normalized)

    return list(dict.fromkeys(filtered))


async def has_visible_verification_submit(page):
    if await find_best_matching_interactive_action(page, VERIFICATION_SUBMIT_TEXTS) is not None:
        return True

    for text in VERIFICATION_SUBMIT_TEXTS:
        if len(normalize_action_text(text)) <= 3:
            continue
        _, candidate = await find_first_visible_containing_text(page, [text])
        if candidate is not None:
            return True
    return False


async def get_verification_container(page, anchor_locator=None):
    if anchor_locator is None:
        return None

    selectors = [
        "xpath=ancestor::*[@role='dialog' or @role='alertdialog'][1]",
        "xpath=ancestor::*[contains(@class,'dialog') or contains(@class,'modal') or contains(@class,'popup') or contains(@class,'verify') or contains(@class,'tooltip')][1]",
    ]

    for selector in selectors:
        try:
            locator = anchor_locator.locator(selector)
            if await locator.count() and await locator.first.is_visible():
                return locator.first
        except Exception:
            continue

    try:
        marker_set = await anchor_locator.evaluate(
            """
            (element, payload) => {
                const attr = payload.attr;
                document.querySelectorAll(`[${attr}]`).forEach((node) => node.removeAttribute(attr));

                const titleKeywords = payload.titleKeywords || [];
                const optionKeywords = payload.optionKeywords || [];
                const submitKeywords = payload.submitKeywords || [];
                let current = element;
                let best = null;
                let bestScore = -1;

                while (current && current !== document.body) {
                    const text = ((current.innerText || current.textContent || "").replace(/\\s+/g, " ").trim());
                    const rect = current.getBoundingClientRect();
                    const style = window.getComputedStyle(current);
                    const role = current.getAttribute("role") || "";
                    const className = `${current.className || ""} ${current.id || ""} ${role}`;
                    const interactiveCount = current.querySelectorAll(
                        "button, [role='button'], a[href], input, textarea, [contenteditable]:not([contenteditable='false']), [role='textbox'], [role='searchbox'], [role='combobox'], [aria-multiline='true']"
                    ).length;

                    let score = 0;
                    if (text) {
                        if (titleKeywords.some((keyword) => text.includes(keyword))) {
                            score += 10;
                        }
                        score += optionKeywords.filter((keyword) => text.includes(keyword)).length * 4;
                        score += submitKeywords.filter((keyword) => text.includes(keyword)).length * 2;
                    }
                    if (rect.width >= 260) {
                        score += 2;
                    }
                    if (rect.height >= 180) {
                        score += 2;
                    }
                    if (style.position === "fixed" || style.position === "sticky") {
                        score += 3;
                    }
                    if (/dialog|modal|popup|verify|captcha|security/i.test(className)) {
                        score += 4;
                    }
                    score += Math.min(interactiveCount, 8);

                    if (score > bestScore) {
                        best = current;
                        bestScore = score;
                    }
                    current = current.parentElement;
                }

                if (!best) {
                    return false;
                }
                best.setAttribute(attr, "1");
                return true;
            }
            """,
            {
                "attr": VERIFICATION_ROOT_MARKER_ATTR,
                "titleKeywords": VERIFICATION_TITLE_TEXTS,
                "optionKeywords": VERIFICATION_OPTION_TEXTS,
                "submitKeywords": VERIFICATION_SUBMIT_TEXTS,
            },
        )
        if marker_set:
            locator = page.locator(f"[{VERIFICATION_ROOT_MARKER_ATTR}='1']")
            if await locator.count() and await locator.last.is_visible():
                return locator.last
    except Exception:
        pass
    return None


async def iter_visible_editable_inputs(locator):
    try:
        candidates = locator.locator(EDITABLE_INPUT_SELECTOR)
        count = await candidates.count()
    except Exception:
        return []

    visible_inputs = []
    for index in range(count):
        candidate = candidates.nth(index)
        try:
            is_vis = await candidate.is_visible()
            meta_payload = await get_editable_meta(candidate)
            if meta_payload is None:
                continue
            if not is_vis:
                input_mode = str(meta_payload.get("inputmode") or "").lower()
                auto_comp = str(meta_payload.get("autocomplete") or "").lower()
                max_len = meta_payload.get("maxlength")
                is_hidden_verify = ("one-time-code" in auto_comp) or (max_len in (4, 6, 8)) or (input_mode in {"numeric", "tel", "decimal"})
                if not is_hidden_verify:
                    continue
            if meta_payload.get("readonly") or meta_payload.get("disabled"):
                continue
            input_type = str(meta_payload.get("type") or "text").lower()
            if input_type in {"hidden", "file", "checkbox", "radio"}:
                continue
            visible_inputs.append(candidate)
        except Exception:
            continue

    return visible_inputs


async def find_focused_editable_input(page):
    for selector in FOCUSED_EDITABLE_SELECTORS:
        try:
            locator = page.locator(selector)
            if await locator.count() and await locator.first.is_visible():
                return locator.first
        except Exception:
            continue

    return None


async def get_verification_search_target(page, anchor_locator=None, option_texts=None):
    container = await get_verification_container(page, anchor_locator)
    if container is not None:
        return container
    return page


# Page layout noise keywords that should be filtered from verification status messages.
# These are common navigation text, brand names, and UI chrome that leak into the message
# when the verification challenge detector scans the entire page.
_STATUS_LINE_NOISE_KEYWORDS = {
    "\u7f51\u5740", "\u6293\u53d6", "\u5bfc\u822a", "\u9996\u9875", "\u767b\u5f55/\u6ce8\u518c", "\u767b\u5f55\u6216\u6ce8\u518c",
    "\u6211\u662f\u521b\u4f5c\u8005", "\u6211\u662fMCN\u673a\u6784", "\u6211\u662fMCN",
    "\u626b\u7801\u767b\u5f55", "\u5bc6\u7801\u767b\u5f55", "\u9a8c\u8bc1\u7801\u767b\u5f55",
    "\u7528\u6237\u534f\u8bae", "\u9690\u79c1\u653f\u7b56", "\u670d\u52a1\u534f\u8bae",
    "\u53d6\u6d88\u767b\u5f55", "\u8fd4\u56de\u767b\u5f55",
    "\u60a8\u7684\u6d4f\u89c8\u5668", "\u4e0b\u8f7d", "\u5b89\u88c5",
}

# Specific platform/brand names to filter
_STATUS_LINE_NOISE_BRANDS = {
    "\u6296\u97f3", "\u5feb\u624b", "\u5c0f\u7ea2\u4e66", "\u89c6\u9891\u53f7", "\u767e\u5bb6\u53f7",
    "\u6296\u97f3\u521b\u4f5c\u8005\u4e2d\u5fc3", "\u5feb\u624b\u521b\u4f5c\u8005",
}


def _clean_verification_status_lines(status_lines, title=None, option_texts=None, input_hints=None):
    """Filter out page layout noise from verification status message lines."""
    if not status_lines:
        return []

    all_known = set()
    if title:
        all_known.add(normalize_action_text(title))
    for text in (option_texts or []):
        all_known.add(normalize_action_text(text))
    for text in (input_hints or []):
        all_known.add(normalize_action_text(text))
    for text in QR_SCANNED_TEXTS:
        all_known.add(normalize_action_text(text))
    for text in QR_EXPIRED_TEXTS:
        all_known.add(normalize_action_text(text))
    for text in QR_REFRESH_TEXTS:
        all_known.add(normalize_action_text(text))

    cleaned = []
    for line in status_lines:
        normalized = normalize_action_text(line)
        if not normalized or len(normalized) < 2:
            continue
        # Skip lines already represented by title/options/hints
        if normalized in all_known:
            continue
        # Skip page layout noise
        if any(keyword in normalized for keyword in _STATUS_LINE_NOISE_KEYWORDS):
            continue
        # Skip pure brand names
        if normalized in _STATUS_LINE_NOISE_BRANDS:
            continue
        # Skip lines that look like concatenated page layout (contain multiple semicolons or are too generic)
        if normalized.count("\u00b7") >= 2 or normalized.count("\uff1b") >= 2:
            continue
        cleaned.append(normalized)

    return cleaned


async def detect_verification_challenge(page):
    title, option_texts, anchor_locator = await get_verification_anchor(page)
    search_target = await get_verification_search_target(page, anchor_locator, option_texts=option_texts)
    option_texts = await collect_visible_option_texts(search_target)
    input_hints = await get_visible_verification_input_hints(search_target)
    status_lines = await get_visible_verification_status_texts(
        search_target,
        exclude_texts=[title, *option_texts, *input_hints],
    )
    option_texts = refine_verification_option_labels(option_texts, title=title, input_hints=input_hints)
    has_submit = await has_visible_verification_submit(search_target)

    if not title and not option_texts and not input_hints:
        return None
    if not title and not option_texts and input_hints and not has_submit:
        return None
    # Clean status_lines: filter out page layout noise (navigation text, brand names, etc.)
    cleaned_status_lines = _clean_verification_status_lines(status_lines, title, option_texts, input_hints)
    payload = {
        "title": title or "需要额外验证",
        "message": "；".join(cleaned_status_lines) if cleaned_status_lines else "检测到登录验证，请在远端页面选择验证方式，必要时输入验证码或密码。",
        "options": option_texts,
        "supportsTextInput": bool(input_hints),
        "inputHints": input_hints,
    }
    signature = f"{payload['title']}|{payload['message']}|{'/'.join(option_texts)}|{'/'.join(input_hints)}|{page.url}"
    return {
        "signature": signature,
        "payload": payload,
    }


async def click_visible_option(page, text):
    locator = page.get_by_text(text, exact=True)
    count = await locator.count()
    for index in range(count):
        candidate = locator.nth(index)
        try:
            if await candidate.is_visible():
                try:
                    await candidate.scroll_into_view_if_needed()
                except Exception:
                    pass
                try:
                    await candidate.click(force=True, timeout=2000)
                    return True
                except Exception:
                    clicked = await candidate.evaluate(
                        """
                        (element, selector) => {
                            const target = element.closest(
                                selector
                            ) || element;
                            const PointerCtor = window.PointerEvent || window.MouseEvent;
                            const fire = (Ctor, type) => {
                                target.dispatchEvent(new Ctor(type, {
                                    bubbles: true,
                                    cancelable: true,
                                    view: window,
                                }));
                            };
                            if (typeof target.focus === "function") {
                                target.focus();
                            }
                            if (typeof target.click === "function") {
                                target.click();
                            }
                            fire(PointerCtor, "pointerdown");
                            fire(window.MouseEvent, "mousedown");
                            fire(PointerCtor, "pointerup");
                            fire(window.MouseEvent, "mouseup");
                            fire(window.MouseEvent, "click");
                            return true;
                        }
                        """
                        ,
                        INTERACTIVE_ACTION_CLOSEST_SELECTOR,
                    )
                    if clicked:
                        return True
        except Exception:
            continue
    return False


async def get_interactive_action_meta(candidate):
    try:
        meta = await candidate.evaluate(
            """
            (element) => {
                const values = [
                    element.innerText,
                    element.textContent,
                    element.value,
                    element.getAttribute("aria-label"),
                    element.getAttribute("title"),
                ];
                const labels = Array.from(
                    new Set(
                        values
                            .filter(Boolean)
                            .flatMap((value) => String(value).split("\\n"))
                            .map((value) => value.replace(/\\s+/g, " ").trim())
                            .filter(Boolean)
                    )
                );
                return {
                    labels,
                    tag: (element.tagName || "").toLowerCase(),
                    type: (element.getAttribute("type") || "").toLowerCase(),
                    role: (element.getAttribute("role") || "").toLowerCase(),
                    className: `${element.className || ""} ${element.id || ""}`.toLowerCase(),
                    disabled: Boolean(
                        element.disabled
                        || element.getAttribute("disabled") !== null
                        || element.getAttribute("aria-disabled") === "true"
                    ),
                };
            }
            """
        )
    except Exception:
        return None

    if not isinstance(meta, dict):
        return None
    meta["labels"] = [normalize_action_text(label) for label in meta.get("labels") or [] if normalize_action_text(label)]
    return meta


def score_interactive_action_meta(meta, texts):
    if not meta or meta.get("disabled"):
        return -1

    labels = meta.get("labels") or []
    best_score = -1
    for label in labels:
        for text in texts:
            best_score = max(best_score, score_action_label_match(label, text))

    if best_score < 0:
        return -1

    tag = str(meta.get("tag") or "").lower()
    action_type = str(meta.get("type") or "").lower()
    role = str(meta.get("role") or "").lower()
    class_name = str(meta.get("className") or "").lower()

    if tag == "button":
        best_score += 12
    elif action_type == "submit":
        best_score += 10
    elif role == "button":
        best_score += 8

    if any(keyword in class_name for keyword in ("submit", "confirm", "verify", "login", "next")):
        best_score += 4

    # Penalize container elements that have contradicting labels (e.g., both "取消" and "验证").
    # These are usually wrapper divs, not the actual clickable button.
    cancel_keywords = ("取消", "关闭", "返回")
    normalized_labels = [normalize_action_text(label) for label in labels if normalize_action_text(label)]
    has_cancel = any(any(keyword in label for keyword in cancel_keywords) for label in normalized_labels)
    has_match = any(any(normalize_action_text(text) in label for text in texts) for label in normalized_labels)
    if has_cancel and has_match and len(normalized_labels) > 1:
        best_score -= 30

    return best_score


async def find_best_matching_interactive_action(target, texts):
    try:
        locator = target.locator(INTERACTIVE_ACTION_SELECTOR)
        count = await locator.count()
    except Exception:
        return None

    best_candidate = None
    best_score = -1
    for index in range(min(count, 80)):
        candidate = locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
        except Exception:
            continue

        meta = await get_interactive_action_meta(candidate)
        score = score_interactive_action_meta(meta, texts)
        if score > best_score:
            best_candidate = candidate
            best_score = score

    return best_candidate


async def click_best_matching_interactive_action(target, texts):
    candidate = await find_best_matching_interactive_action(target, texts)
    if candidate is None:
        return False

    try:
        try:
            await candidate.scroll_into_view_if_needed()
        except Exception:
            pass
        await candidate.click(force=True, timeout=2000)
        return True
    except Exception:
        try:
            clicked = await candidate.evaluate(
                """
                (element, selector) => {
                    const target = element.closest(
                        selector
                    ) || element;
                    const PointerCtor = window.PointerEvent || window.MouseEvent;
                    const fire = (Ctor, type) => {
                        target.dispatchEvent(new Ctor(type, {
                            bubbles: true,
                            cancelable: true,
                            view: window,
                        }));
                    };
                    if (typeof target.focus === "function") {
                        target.focus();
                    }
                    if (typeof target.click === "function") {
                        target.click();
                    }
                    fire(PointerCtor, "pointerdown");
                    fire(window.MouseEvent, "mousedown");
                    fire(PointerCtor, "pointerup");
                    fire(window.MouseEvent, "mouseup");
                    fire(window.MouseEvent, "click");
                    return true;
                }
                """
                ,
                INTERACTIVE_ACTION_CLOSEST_SELECTOR,
            )
            return bool(clicked)
        except Exception:
            return False


async def get_verification_targets(page):
    _, _, anchor_locator = await get_verification_anchor(page)
    container = await get_verification_container(page, anchor_locator)
    targets = []
    for target in (container, page):
        if target is None:
            continue
        if any(target is existing for existing in targets):
            continue
        targets.append(target)
    return targets


async def click_visible_partial_option(target, texts):
    _, candidate = await find_first_visible_containing_text(target, texts)
    if candidate is None:
        return False
    try:
        try:
            await candidate.scroll_into_view_if_needed()
        except Exception:
            pass
        await candidate.click(force=True, timeout=2000)
        return True
    except Exception:
        try:
            clicked = await candidate.evaluate(
                """
                (element, selector) => {
                    const target = element.closest(
                        selector
                    ) || element;
                    const PointerCtor = window.PointerEvent || window.MouseEvent;
                    const fire = (Ctor, type) => {
                        target.dispatchEvent(new Ctor(type, {
                            bubbles: true,
                            cancelable: true,
                            view: window,
                        }));
                    };
                    if (typeof target.focus === "function") {
                        target.focus();
                    }
                    if (typeof target.click === "function") {
                        target.click();
                    }
                    fire(PointerCtor, "pointerdown");
                    fire(window.MouseEvent, "mousedown");
                    fire(PointerCtor, "pointerup");
                    fire(window.MouseEvent, "mouseup");
                    fire(window.MouseEvent, "click");
                    return true;
                }
                """
                ,
                INTERACTIVE_ACTION_CLOSEST_SELECTOR,
            )
            return bool(clicked)
        except Exception:
            return False


async def get_editable_meta(candidate):
    try:
        input_type = (await candidate.get_attribute("type") or "text").lower()
        placeholder = " ".join(
            (
                (await candidate.get_attribute("placeholder"))
                or (await candidate.get_attribute("aria-placeholder"))
                or (await candidate.get_attribute("data-placeholder"))
                or ""
            ).split()
        )
        aria_label = " ".join(((await candidate.get_attribute("aria-label")) or "").split())
        name = " ".join(((await candidate.get_attribute("name")) or "").split())
        autocomplete = " ".join(((await candidate.get_attribute("autocomplete")) or "").split()).lower()
        inputmode = " ".join(((await candidate.get_attribute("inputmode")) or "").split()).lower()
        class_name = " ".join(((await candidate.get_attribute("class")) or "").split()).lower()
        element_id = " ".join(((await candidate.get_attribute("id")) or "").split()).lower()
        maxlength_raw = (await candidate.get_attribute("maxlength")) or ""
        readonly = await candidate.get_attribute("readonly")
        disabled = await candidate.get_attribute("disabled")
        try:
            text = " ".join(
                (
                    await candidate.evaluate(
                        """
                        (element) => [
                            element.innerText || "",
                            element.textContent || "",
                        ].join(" ")
                        """
                    )
                ).split()
            )
        except Exception:
            text = ""
    except Exception:
        return None

    try:
        maxlength = int(str(maxlength_raw).strip()) if str(maxlength_raw).strip() else None
    except Exception:
        maxlength = None

    return {
        "type": input_type,
        "placeholder": placeholder,
        "ariaLabel": aria_label,
        "name": name,
        "autocomplete": autocomplete,
        "inputmode": inputmode,
        "className": class_name,
        "id": element_id,
        "text": text,
        "maxlength": maxlength,
        "readonly": readonly is not None,
        "disabled": disabled is not None,
    }


async def get_editable_value(candidate):
    try:
        value = await candidate.evaluate(
            """
            (element) => {
                if (!element) {
                    return "";
                }
                if (element.isContentEditable) {
                    return (element.innerText || element.textContent || "").trim();
                }
                if ("value" in element) {
                    return element.value || "";
                }
                return (element.textContent || "").trim();
            }
            """
        )
    except Exception:
        return ""
    return str(value or "").strip()


async def dispatch_editable_value(candidate, text):
    try:
        await candidate.evaluate(
            """
            (element, value) => {
                const emit = (eventName) => {
                    element.dispatchEvent(new Event(eventName, { bubbles: true }));
                };
                const emitKeyboard = (eventName) => {
                    element.dispatchEvent(new KeyboardEvent(eventName, {
                        key: "Enter",
                        bubbles: true,
                        cancelable: true,
                    }));
                };

                if (element.isContentEditable) {
                    element.focus();
                    element.textContent = value;
                    emit("input");
                    emit("change");
                    return;
                }

                if ("value" in element) {
                    const prototype = element.tagName === "TEXTAREA"
                        ? window.HTMLTextAreaElement.prototype
                        : window.HTMLInputElement.prototype;
                    const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
                    if (descriptor && typeof descriptor.set === "function") {
                        descriptor.set.call(element, value);
                    } else {
                        element.value = value;
                    }
                    element.focus();
                    emit("input");
                    emit("change");
                    emitKeyboard("keyup");
                }
            }
            """,
            text,
        )
        return True
    except Exception:
        return False


async def score_editable_input(candidate, desired_text=None):
    meta_payload = await get_editable_meta(candidate)
    if meta_payload is None:
        return -1
    if meta_payload.get("readonly") or meta_payload.get("disabled"):
        return -1

    input_type = str(meta_payload.get("type") or "text").lower()
    meta = " ".join(
        filter(
            None,
            [
                meta_payload.get("placeholder"),
                meta_payload.get("ariaLabel"),
                meta_payload.get("name"),
                meta_payload.get("className"),
                meta_payload.get("id"),
                meta_payload.get("text"),
            ],
        )
    ).strip()
    score = 0
    if input_type == "password" or "密码" in meta:
        score += 4
    if "验证码" in meta or "短信" in meta:
        score += 6
    if "手机" in meta or "手机号" in meta:
        score += 2
    if "one-time-code" in str(meta_payload.get("autocomplete") or ""):
        score += 10
    if str(meta_payload.get("inputmode") or "") in {"numeric", "decimal", "tel"}:
        score += 6

    maxlength = meta_payload.get("maxlength")
    if isinstance(maxlength, int) and 1 <= maxlength <= 8:
        score += 3

    text = str(desired_text or "").strip()
    if text:
        if text.isdigit() and 4 <= len(text) <= 8:
            if "验证码" in meta or "短信" in meta:
                score += 8
            if "one-time-code" in str(meta_payload.get("autocomplete") or ""):
                score += 12
            if str(meta_payload.get("inputmode") or "") in {"numeric", "decimal", "tel"}:
                score += 8
            if isinstance(maxlength, int) and maxlength == len(text):
                score += 8
            if "手机" in meta or "手机号" in meta:
                score -= 6
        elif text.isdigit() and len(text) >= 11:
            if "手机" in meta or "手机号" in meta:
                score += 8
        else:
            if input_type == "password" or "密码" in meta:
                score += 8
    return score


async def find_first_editable_input(page, desired_text=None):
    focused_input = await find_focused_editable_input(page)
    if focused_input is not None:
        return focused_input

    _, _, anchor_locator = await get_verification_anchor(page)
    container = await get_verification_container(page, anchor_locator)
    best_input = None
    best_score = -1
    if container is not None:
        container_inputs = await iter_visible_editable_inputs(container)
        for candidate in container_inputs:
            score = await score_editable_input(candidate, desired_text)
            if score > best_score:
                best_input = candidate
                best_score = score
        if best_input is not None:
            return best_input

    page_inputs = await iter_visible_editable_inputs(page)
    for candidate in page_inputs:
        score = await score_editable_input(candidate, desired_text)
        if score > best_score:
            best_input = candidate
            best_score = score
    if best_input is not None:
        return best_input

    return None


async def click_submit_action(page):
    for target in await get_verification_targets(page):
        if await click_best_matching_interactive_action(target, VERIFICATION_SUBMIT_TEXTS):
            return True
        for text in VERIFICATION_SUBMIT_TEXTS:
            if len(normalize_action_text(text)) <= 3:
                if await click_visible_option(target, text):
                    return True
                continue
            if await click_visible_option(target, text):
                return True
            if await click_visible_partial_option(target, [text]):
                return True
    return False


async def get_verification_signature_snapshot(page):
    if page is None or page.is_closed():
        return None
    try:
        challenge = await detect_verification_challenge(page)
    except Exception:
        return None
    if not challenge:
        return None
    return challenge.get("signature")


async def did_verification_view_change(page, before_signature=None, before_url=""):
    if page is None or page.is_closed():
        return True
    current_url = str(getattr(page, "url", "") or "")
    if before_url and current_url and current_url != before_url:
        return True
    current_signature = await get_verification_signature_snapshot(page)
    if before_signature is None:
        return current_signature is None
        
    if current_signature != before_signature:
        def strip_timer(sig):
            return re.sub(r'\d+\s*(?:s|秒)后(?:重新)?(?:发送|获取)(?:验证码)?', 'TIMER_PLACEHOLDER', str(sig or ""))
            
        if strip_timer(current_signature) != strip_timer(before_signature):
            return True
            
    return False


async def dump_verification_debug_snapshot(page, label="verification", target=None, extra_payload=None):
    if page is None or page.is_closed():
        return None

    if target is None:
        try:
            target = await get_verification_search_target(page)
        except Exception:
            target = page

    try:
        html = await target.evaluate(
            """
            (element) => element ? (element.outerHTML || element.innerHTML || "") : ""
            """
        )
    except Exception as exc:
        html = f"<!-- failed to capture verification html: {exc} -->"

    candidates = []
    try:
        locator = target.locator(INTERACTIVE_ACTION_SELECTOR)
        count = await locator.count()
    except Exception:
        count = 0
        locator = None

    for index in range(min(count, 40)):
        candidate = locator.nth(index)
        try:
            if not await candidate.is_visible():
                continue
        except Exception:
            continue
        meta = await get_interactive_action_meta(candidate)
        if not meta:
            continue
        candidates.append(meta)

    try:
        screenshot_bytes = await target.screenshot(type="png")
    except Exception:
        screenshot_bytes = None

    LOGIN_DEBUG_LOG_DIR.mkdir(parents=True, exist_ok=True)
    base_name = f"verification_{label}_latest"
    html_path = LOGIN_DEBUG_LOG_DIR / f"{base_name}.html"
    json_path = LOGIN_DEBUG_LOG_DIR / f"{base_name}.json"
    screenshot_path = LOGIN_DEBUG_LOG_DIR / f"{base_name}.png"

    try:
        html_path.write_text(str(html or ""), encoding="utf-8")
    except Exception:
        pass

    payload = {
        "label": label,
        "url": str(getattr(page, "url", "") or ""),
        "interactiveCandidates": candidates,
        "extra": extra_payload or {},
    }
    try:
        json_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    except Exception:
        pass

    if screenshot_bytes:
        try:
            screenshot_path.write_bytes(screenshot_bytes)
        except Exception:
            pass

    return {
        "htmlPath": str(html_path),
        "jsonPath": str(json_path),
        "screenshotPath": str(screenshot_path),
    }


async def submit_verification_action(page, input_locator=None):
    before_url = str(getattr(page, "url", "") or "")
    before_signature = await get_verification_signature_snapshot(page)
    dispatched = False

    await dump_verification_debug_snapshot(
        page,
        label="submit",
        extra_payload={
            "beforeSignature": before_signature,
            "beforeUrl": before_url,
        },
    )

    async def _after_dispatch_pause():
        for _ in range(5):
            await asyncio.sleep(0.5)
            if await did_verification_view_change(page, before_signature=before_signature, before_url=before_url):
                return True
        return False

    async def _try_click():
        return await click_submit_action(page)

    async def _try_input_enter():
        if input_locator is None:
            return False
        try:
            await input_locator.click(force=True)
        except Exception:
            pass
        try:
            await input_locator.press("Enter")
            return True
        except Exception:
            return False

    # Only try safe strategies: click the submit button, or press Enter on the input field.
    for name, strategy in (
        ("click_submit", _try_click),
        ("input_enter", _try_input_enter),
    ):
        try:
            applied = await strategy()
            if applied:
                dispatched = True
                break
        except Exception:
            pass
            
    if dispatched:
        if await _after_dispatch_pause():
            log_throttled(
                login_logger,
                "INFO",
                f"login.submit.success:{id(page)}",
                1,
                "verification submit completed current_url={}",
                page.url if not page.is_closed() else "closed",
            )
            return True

        log_throttled(
            login_logger,
            "INFO",
            f"login.submit.no_effect:{id(page)}:{name}",
            1,
            "verification submit had no visible effect via strategy={} current_url={} signature={}",
            name,
            page.url if not page.is_closed() else "closed",
            before_signature,
        )

    return dispatched


async def click_verification_option(page, text):
    for target in await get_verification_targets(page):
        if await click_best_matching_interactive_action(target, [text]):
            return True
        if await click_visible_option(target, text):
            return True
        if await click_visible_partial_option(target, [text]):
            return True
    return False


def get_select_all_shortcut():
    return "Meta+A" if sys.platform == "darwin" else "Control+A"


def get_login_browser_options(command_queue=None, extra_args=None):
    force_headless = bool(command_queue) and sys.platform.startswith("linux")
    force_headed = bool(command_queue) and os.name == "nt"
    if force_headless:
        login_logger.info("using headless browser for remote login on linux")
    if force_headed:
        login_logger.info("using headed browser for remote login on windows")
    if force_headless:
        return get_browser_options(headless=True, extra_args=extra_args)
    if force_headed:
        return get_browser_options(headless=False, extra_args=extra_args)
    return get_browser_options(headless=None, extra_args=extra_args)


async def fill_input_like_user(page, input_locator, text):
    try:
        await input_locator.scroll_into_view_if_needed()
    except Exception:
        pass

    async def _verify():
        current_value = await get_editable_value(input_locator)
        if str(current_value) == str(text):
            return True
        try:
            meta = await get_editable_meta(input_locator)
            if meta and meta.get("maxlength") == 1 and current_value == str(text)[0]:
                return True
        except Exception:
            pass
        return False

    async def _prepare_focus():
        try:
            await input_locator.click(force=True)
        except Exception:
            pass
        try:
            await input_locator.focus()
        except Exception:
            pass
        try:
            await page.keyboard.press(get_select_all_shortcut())
            await page.keyboard.press("Delete")
        except Exception:
            pass

    async def _fill_direct():
        await _prepare_focus()
        await input_locator.fill(text)

    async def _type_direct():
        await _prepare_focus()
        try:
            await input_locator.type(text, delay=80)
        except Exception:
            await page.keyboard.type(text, delay=80)

    async def _dispatch_value():
        await _prepare_focus()
        dispatched = await dispatch_editable_value(input_locator, text)
        if not dispatched:
            raise RuntimeError("dispatch_editable_value_failed")

    async def _keyboard_type():
        await _prepare_focus()
        for char in str(text):
            await page.keyboard.press(char)
            await asyncio.sleep(0.05)

    for strategy in (_fill_direct, _type_direct, _dispatch_value, _keyboard_type):
        try:
            await strategy()
            await asyncio.sleep(0.2)
            if await _verify():
                return True
        except Exception:
            continue
    return False


def push_structured_status(status_queue, command_queue, event_type, payload):
    if status_queue is not None and command_queue is not None:
        status_queue.put({
            "type": event_type,
            "payload": payload,
        })


def push_login_failed_status(status_queue, command_queue, message):
    normalized_message = str(message or "").strip() or "登录失败，请重试"
    push_structured_status(
        status_queue,
        command_queue,
        "login_failed",
        {
            "message": normalized_message,
        },
    )


async def detect_login_qr_state(page, qr_locator=None, qr_action_root=None):
    qr_signature = await get_locator_visual_signature_if_visible(qr_locator)
    expired_text = None
    scanned_text = None
    for target in [target for target in (qr_action_root, page) if target is not None]:
        expired_text, _ = await find_first_visible_containing_text(target, QR_EXPIRED_TEXTS)
        if expired_text:
            break
    for target in [target for target in (qr_action_root, page) if target is not None]:
        scanned_text, _ = await find_first_visible_containing_text(target, QR_SCANNED_TEXTS)
        if scanned_text:
            break
    return {
        "qrSignature": qr_signature,
        "isExpired": bool(expired_text),
        "expiredText": expired_text,
        "isScanned": bool(scanned_text),
        "scannedText": scanned_text,
    }


async def sync_login_qr_state(status_queue, command_queue, page, qr_locator=None, qr_action_root=None, tracker=None):
    if tracker is None:
        tracker = {}

    qr_state = await detect_login_qr_state(page, qr_locator=qr_locator, qr_action_root=qr_action_root)
    now = asyncio.get_running_loop().time()
    qr_signature = qr_state.get("qrSignature")
    should_capture_qr = False
    if qr_signature and qr_signature != tracker.get("lastQrSignature"):
        should_capture_qr = True
    elif qr_signature and not tracker.get("lastQrData"):
        should_capture_qr = True
    elif qr_signature and now - float(tracker.get("lastQrSnapshotAt") or 0.0) >= QR_SNAPSHOT_MIN_INTERVAL_SECONDS:
        should_capture_qr = True

    qr_data = tracker.get("lastQrData")
    if should_capture_qr:
        latest_qr_data = await locator_to_data_url_if_visible(qr_locator)
        tracker["lastQrSnapshotAt"] = now
        if latest_qr_data:
            qr_data = latest_qr_data
            if qr_data != tracker.get("lastQrData") or qr_signature != tracker.get("lastQrSignature"):
                tracker["lastQrData"] = qr_data
                tracker["lastQrSignature"] = qr_signature
                tracker["isExpired"] = False
                tracker["isScanned"] = False
                push_structured_status(
                    status_queue,
                    command_queue,
                    "qr_updated",
                    {
                        "message": "本地登录二维码已更新，请使用最新二维码扫码。",
                        "qrData": qr_data,
                    },
                )
        elif qr_signature != tracker.get("lastQrSignature"):
            tracker["lastQrSignature"] = qr_signature

    is_expired = bool(qr_state.get("isExpired"))
    if is_expired and not tracker.get("isExpired"):
        tracker["isExpired"] = True
        push_structured_status(
            status_queue,
            command_queue,
            "qr_expired",
            {
                "message": "本地登录二维码已过期，请刷新二维码。",
                "qrData": qr_data or tracker.get("lastQrData"),
            },
        )
    elif not is_expired and tracker.get("isExpired") and qr_data:
        tracker["isExpired"] = False

    is_scanned = bool(qr_state.get("isScanned"))
    if is_scanned and not tracker.get("isScanned"):
        tracker["isScanned"] = True
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {
                "message": qr_state.get("scannedText") or "已扫码，请在手机上确认登录。",
            },
        )
    elif not is_scanned and tracker.get("isScanned"):
        if qr_signature and qr_signature != tracker.get("lastQrSignature"):
            tracker["isScanned"] = False

    return qr_state


async def apply_remote_action(page, action, status_queue=None, command_queue=None, qr_action_root=None):
    if not action:
        return False

    action_type = action.get("actionType") or ""
    payload = action.get("payload") or {}

    login_logger.info(
        "apply_remote_action action_type={} payload={} current_url={}",
        action_type,
        {k: v for k, v in payload.items()} if isinstance(payload, dict) else payload,
        page.url if not page.is_closed() else "closed",
    )

    if action_type in {"select_option", "click_text"}:
        target_text = str(payload.get("text") or payload.get("optionText") or payload.get("option") or "").strip()
        if not target_text:
            login_logger.info("apply_remote_action select_option skipped - no target_text")
            return False
        result = await click_verification_option(page, target_text)
        login_logger.info(
            "apply_remote_action select_option target_text={} direct_click_result={} current_url={}",
            target_text, result, page.url if not page.is_closed() else "closed",
        )
        if not result and any(keyword in target_text for keyword in VERIFICATION_CODE_ACTION_TEXTS):
            for fallback_text in VERIFICATION_CODE_ACTION_TEXTS:
                if fallback_text == target_text:
                    continue
                result = await click_verification_option(page, fallback_text)
                if result:
                    login_logger.info(
                        "apply_remote_action select_option fallback_text={} click_result=True current_url={}",
                        fallback_text, page.url if not page.is_closed() else "closed",
                    )
                    break
        if result:
            push_structured_status(
                status_queue,
                command_queue,
                "log",
                {"message": f"远端已选择验证方式：{target_text}"},
            )
        return result

    if action_type == "fill_text":
        text = str(payload.get("text") or "").strip()
        if not text:
            return False
        input_locator = await find_first_editable_input(page, text)
        if input_locator is None:
            return False
        filled = await fill_input_like_user(page, input_locator, text)
        if not filled:
            return False
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": "远端已向本地验证框填入内容"},
        )
        return True

    if action_type == "press_key":
        key = str(payload.get("key") or "Enter").strip() or "Enter"
        input_locator = await find_first_editable_input(page)
        if input_locator is not None:
            try:
                await input_locator.click(force=True)
                await input_locator.press(key)
            except Exception:
                await page.keyboard.press(key)
        else:
            await page.keyboard.press(key)
        if key.lower() == "enter":
            await asyncio.sleep(0.2)
            if await detect_verification_challenge(page):
                await click_submit_action(page)
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": f"远端已向本地浏览器发送按键：{key}"},
        )
        return True

    if action_type == "fill_text_and_submit":
        text = str(payload.get("text") or "").strip()
        if not text:
            return False
        input_locator = await find_first_editable_input(page, text)
        if input_locator is None:
            return False
        filled = await fill_input_like_user(page, input_locator, text)
        if not filled:
            return False
        await asyncio.sleep(0.2)
        submitted = await submit_verification_action(page, input_locator=input_locator)
        if submitted:
            await asyncio.sleep(0.3)
            push_structured_status(
                status_queue,
                command_queue,
                "log",
                {"message": "远端已发送验证码并尝试提交验证"},
            )
            return True
        return False

    if action_type == "refresh_qr":
        targets = [target for target in (qr_action_root, page) if target is not None]
        for target in targets:
            refreshed = await click_visible_partial_option(target, QR_REFRESH_TEXTS)
            if refreshed:
                push_structured_status(
                    status_queue,
                    command_queue,
                    "log",
                    {"message": "远端已请求本地 SAU 刷新二维码"},
                )
                return True
        return False

    if action_type in {"cancel_session", "cancel_login"}:
        push_structured_status(
            status_queue,
            command_queue,
            "log",
            {"message": "远端已取消当前登录会话，本地 SAU 正在关闭登录窗口。"},
        )
        try:
            await page.close()
        except Exception:
            pass
        raise LoginCancelled()

    return False


async def drain_remote_actions(page, command_queue, status_queue=None, qr_action_root=None):
    if command_queue is None:
        return False

    handled = False
    while True:
        try:
            action = command_queue.get_nowait()
        except Empty:
            break

        action_type = str(action.get("actionType") or "").strip()
        if action_type in {"cancel_session", "cancel_login"}:
            login_logger.info("drain_remote_actions processing explicit cancellation action")
            try:
                await page.close()
            except Exception:
                pass
            raise LoginCancelled()

        applied = await apply_remote_action(page, action, status_queue, command_queue, qr_action_root=qr_action_root)
        if applied:
            handled = True

    return handled


async def wait_for_login_result(
    page,
    original_url,
    url_changed_event,
    status_queue,
    command_queue=None,
    timeout=200,
    qr_locator=None,
    qr_action_root=None,
    initial_qr_data=None,
    verification_timeout=900,
    verification_settle_seconds=2.0,
    success_validator=None,
    success_check_interval=2.5,
):
    loop = asyncio.get_running_loop()
    deadline = loop.time() + timeout
    last_signature = None
    qr_tracker = {"lastQrData": initial_qr_data, "isExpired": False, "isScanned": False}
    qr_hidden_since = None
    qr_was_visible_at = None
    verification_started_at = None
    verification_cleared_since = None
    verification_observed = False
    last_success_check_at = 0.0

    while loop.time() < deadline:
        now = loop.time()
        if page.is_closed():
            login_logger.info("login page closed after qr flow original_url={}", original_url)
            return "cancelled"

        try:
            qr_state = await sync_login_qr_state(
                status_queue,
                command_queue,
                page,
                qr_locator=qr_locator,
                qr_action_root=qr_action_root,
                tracker=qr_tracker,
            )
            qr_visible = await is_locator_visible(qr_locator)
            if qr_visible:
                qr_was_visible_at = now
            qr_phase_waiting_scan = bool(
                qr_locator is not None and qr_visible and not qr_state.get("isScanned") and not qr_state.get("isExpired")
            )
            # When QR just disappeared (user scanned, avatar replaces QR image),
            # suppress weak verification challenges during the grace period.
            # This prevents the right-side login form "获取验证码" from being
            # misidentified as a real verification challenge.
            qr_phase_pending_confirm = bool(
                qr_locator is not None
                and not qr_visible
                and not qr_state.get("isExpired")
                and qr_was_visible_at is not None
                and now - qr_was_visible_at < QR_HIDDEN_GRACE_SECONDS
                and verification_started_at is None
            )
            challenge = await detect_verification_challenge(page)
            if challenge and (qr_phase_waiting_scan or qr_phase_pending_confirm) and not is_strong_verification_challenge(challenge):
                log_throttled(
                    login_logger,
                    "INFO",
                    f"login.wait.weak_verification:{original_url}",
                    5,
                    "ignoring weak verification signal while qr phase active original_url={} current_url={} title={} options={} pending_confirm={}",
                    original_url,
                    page.url,
                    (challenge.get("payload") or {}).get("title"),
                    (challenge.get("payload") or {}).get("options"),
                    qr_phase_pending_confirm,
                )
                challenge = None
            if challenge:
                qr_phase_waiting_scan = False
        except Exception as exc:
            if is_transient_login_page_error(exc):
                deadline = max(deadline, loop.time() + 10)
                log_throttled(
                    login_logger,
                    "INFO",
                    f"login.wait.transient:{id(page)}",
                    5,
                    "login page changed during verification original_url={} current_url={} waiting_for_page_settle=true error={}",
                    original_url,
                    page.url if not page.is_closed() else "closed",
                    exc,
                )
                await asyncio.sleep(0.3)
                continue
            raise

        if challenge:
            verification_observed = True
            if verification_started_at is None:
                verification_started_at = now
                deadline = max(deadline, now + max(verification_timeout, timeout))
                login_logger.info(
                    "login verification challenge detected original_url={} current_url={} timeout_extended_to={}s",
                    original_url,
                    page.url,
                    max(verification_timeout, timeout),
                )
            verification_cleared_since = None
            if challenge["signature"] != last_signature:
                await dump_verification_debug_snapshot(
                    page,
                    label="challenge",
                    extra_payload={
                        "signature": challenge["signature"],
                        "payload": challenge["payload"],
                    },
                )
                push_structured_status(status_queue, command_queue, "verification_required", challenge["payload"])
                last_signature = challenge["signature"]
            try:
                handled = await drain_remote_actions(page, command_queue, status_queue, qr_action_root=qr_action_root)
            except LoginCancelled:
                login_logger.info("login cancelled during verification original_url={} current_url={}", original_url, page.url)
                return "cancelled"
            if handled:
                deadline = max(deadline, loop.time() + 180)
                last_signature = None
                await asyncio.sleep(0.5)
                continue
            await asyncio.sleep(0.5)
            continue

        verification_active = verification_started_at is not None
        if verification_active and verification_cleared_since is None:
            verification_cleared_since = now
            login_logger.info(
                "login verification challenge cleared original_url={} current_url={} waiting_for_completion=true",
                original_url,
                page.url,
            )
        verification_settled = not verification_active or (
            verification_cleared_since is not None and now - verification_cleared_since >= verification_settle_seconds
        )

        if (
            success_validator is not None
            and verification_observed
            and verification_settled
            and now - last_success_check_at >= max(success_check_interval, 0.5)
        ):
            last_success_check_at = now
            try:
                if await success_validator(page):
                    login_logger.info(
                        "login success validator passed original_url={} current_url={}",
                        original_url,
                        page.url,
                    )
                    return True
            except Exception:
                pass

        if url_changed_event.is_set() or page.url != original_url:
            if not verification_settled:
                await asyncio.sleep(0.5)
                continue
            if success_validator is not None and verification_observed:
                try:
                    if await success_validator(page):
                        login_logger.info(
                            "login success validator passed after navigation original_url={} current_url={}",
                            original_url,
                            page.url,
                        )
                        return True
                except Exception:
                    pass
                deadline = max(deadline, loop.time() + max(success_check_interval * 4, 30))
                login_logger.info(
                    "login navigation observed before login completion original_url={} current_url={} waiting_for_success_validator=true",
                    original_url,
                    page.url,
                )
                await asyncio.sleep(0.5)
                continue
            login_logger.info("login navigation detected original_url={} current_url={}", original_url, page.url)
            return True

        if qr_locator is not None and not qr_visible and not qr_state.get("isExpired"):
            if qr_state.get("isScanned"):
                deadline = max(deadline, loop.time() + 180)
                qr_hidden_since = None
            elif verification_observed:
                deadline = max(deadline, loop.time() + 30)
                qr_hidden_since = None
            elif not verification_settled:
                qr_hidden_since = None
            elif qr_hidden_since is None:
                qr_hidden_since = now
            elif now - qr_hidden_since >= 1.5:
                login_logger.info("login qr disappeared and stayed hidden original_url={} current_url={}", original_url, page.url)
                return True
        else:
            qr_hidden_since = None

        last_signature = None
        try:
            handled = await drain_remote_actions(page, command_queue, status_queue, qr_action_root=qr_action_root)
        except LoginCancelled:
            login_logger.info("login cancelled original_url={} current_url={}", original_url, page.url)
            return "cancelled"
        if handled:
            if verification_active:
                deadline = max(deadline, loop.time() + 180)
            await asyncio.sleep(0.5)
            continue

        if verification_active and verification_settled:
            verification_started_at = None
            verification_cleared_since = None

        await asyncio.sleep(0.5)

    return False


async def persist_login_state_with_retry(
    context,
    account_type,
    account_name,
    platform_label,
    verify_timeout=30,
    verify_interval=2,
    page=None,
    status_queue=None,
    command_queue=None,
):
    uuid_v1 = uuid.uuid1()
    file_name = f"{uuid_v1}.json"
    settle_deadline = asyncio.get_running_loop().time() + 1.5
    while asyncio.get_running_loop().time() < settle_deadline:
        await drain_remote_actions(page, command_queue, status_queue)
        await asyncio.sleep(0.2)

    deadline = asyncio.get_running_loop().time() + max(verify_timeout, verify_interval)
    attempt = 0
    last_error = None
    last_verification_signature = None
    page_closed_logged = False

    while asyncio.get_running_loop().time() < deadline:
        attempt += 1
        try:
            await drain_remote_actions(page, command_queue, status_queue)
            page_available = page is not None and not page.is_closed()
            if page is not None and not page_available:
                if not page_closed_logged:
                    login_logger.warning(
                        "{} login page closed before storage verification completed account_name={} attempt={} continuing_with_storage_state=true",
                        platform_label,
                        account_name,
                        attempt,
                    )
                    page_closed_logged = True
                page = None
            if page_available:
                challenge = await detect_verification_challenge(page)
                if challenge:
                    deadline = max(deadline, asyncio.get_running_loop().time() + 120)
                    if challenge["signature"] != last_verification_signature:
                        push_structured_status(status_queue, command_queue, "verification_required", challenge["payload"])
                        last_verification_signature = challenge["signature"]
                    await asyncio.sleep(0.5)
                    continue
            last_verification_signature = None

            storage_state = await context.storage_state()
            if page is not None and not page.is_closed():
                live_result = await validate_login_completion_detail(
                    account_type,
                    page,
                    settle_seconds=0.8,
                    retries=2,
                    retry_delay_seconds=0.8,
                )
                if bool(live_result.get("ok")):
                    save_login_account(account_type, account_name, file_name, 1, storage_state=storage_state)
                    login_logger.info(
                        "{} login cookie saved from active page account_name={} cookie_file={} attempts={}",
                        platform_label,
                        account_name,
                        file_name,
                        attempt,
                    )
                    return file_name
                last_error = str(live_result.get("message") or "").strip() or last_error

            cookie_result = await check_cookie_detail(account_type, storage_state, headless=True)
            if bool(cookie_result.get("ok")):
                save_login_account(account_type, account_name, file_name, 1, storage_state=storage_state)
                login_logger.info(
                    "{} login cookie saved account_name={} cookie_file={} attempts={}",
                    platform_label,
                    account_name,
                    file_name,
                    attempt,
                )
                return file_name
            last_error = str(cookie_result.get("message") or "").strip() or "cookie_invalid"
            login_logger.warning(
                "{} login detected but cookie not ready account_name={} attempt={} message={}",
                platform_label,
                account_name,
                attempt,
                last_error,
            )
        except Exception as exc:
            if is_transient_login_page_error(exc):
                deadline = max(deadline, asyncio.get_running_loop().time() + 10)
                log_throttled(
                    login_logger,
                    "INFO",
                    f"login.persist.transient:{platform_label}:{account_name}",
                    5,
                    "{} login page changed while persisting state account_name={} attempt={} waiting_for_page_settle=true error={}",
                    platform_label,
                    account_name,
                    attempt,
                    exc,
                )
                await asyncio.sleep(0.3)
                continue
            last_error = str(exc)
            login_logger.warning(
                "{} login cookie verify error account_name={} attempt={} error={}",
                platform_label,
                account_name,
                attempt,
                exc,
            )

        sleep_deadline = asyncio.get_running_loop().time() + verify_interval
        while asyncio.get_running_loop().time() < sleep_deadline:
            await drain_remote_actions(page, command_queue, status_queue)
            await asyncio.sleep(0.2)

    login_logger.error(
        "{} login cookie verify failed account_name={} attempts={} last_error={}",
        platform_label,
        account_name,
        attempt,
        last_error,
    )
    raise LoginPersistFailed(
        f"{platform_label} 登录后未能确认本地登录态已生效: {str(last_error or '等待超时').strip()}"
    )


def build_login_success_validator(account_type):
    async def validator(page):
        result = await validate_login_completion_detail(
            account_type,
            page,
            settle_seconds=0.4,
            retries=2,
            retry_delay_seconds=0.4,
        )
        return bool(result.get("ok"))

    return validator

# 抖音登录
async def douyin_cookie_gen(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()
    async with async_playwright() as playwright:
        options = get_login_browser_options(command_queue=command_queue)
        # Make sure to run headed.
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://creator.douyin.com/")
        original_url = page.url
        img_locator = page.get_by_role("img", name="二维码")
        qr_data = await locator_to_data_url(img_locator)
        login_logger.info("douyin qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)
        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
                success_validator=build_login_success_validator(3),
            )
            if login_result == "cancelled":
                login_logger.info("douyin login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("douyin login navigation detected account_name={}", id)
            push_structured_status(
                status_queue,
                command_queue,
                "running",
                {"message": "安全验证通过，正在测试并保存登录配置，此过程通常约 10-15 秒，偶遇网络或平台校验可能长达一分钟，请耐心等待..."},
            )
        except asyncio.TimeoutError:
            login_logger.warning("douyin login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("500")
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                3,
                id,
                "douyin",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("douyin login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        except LoginPersistFailed as exc:
            login_logger.warning("douyin login state persist failed account_name={} error={}", id, exc)
            push_login_failed_status(status_queue, command_queue, str(exc))
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")


# 视频号登录
async def get_tencent_cookie(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()

    async with async_playwright() as playwright:
        options = get_login_browser_options(command_queue=command_queue, extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        # Pause the page, and start recording manually.
        context = await set_init_script(context)
        page = await context.new_page()
        await page.goto("https://channels.weixin.qq.com")
        original_url = page.url

        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        # 等待 iframe 出现（最多等 60 秒）
        iframe_locator = page.frame_locator("iframe").first

        # 获取 iframe 中的第一个 img 元素
        img_locator = iframe_locator.get_by_role("img").first

        qr_data = await locator_to_data_url(img_locator)
        login_logger.info("tencent qr generated account_name={}", id)
        status_queue.put(qr_data)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=iframe_locator,
                initial_qr_data=qr_data,
                success_validator=build_login_success_validator(2),
            )
            if login_result == "cancelled":
                login_logger.info("tencent login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("tencent login navigation detected account_name={}", id)
            push_structured_status(
                status_queue,
                command_queue,
                "running",
                {"message": "安全验证通过，正在测试并保存登录配置，此过程通常约 10-15 秒，偶遇网络或平台校验可能长达一分钟，请耐心等待..."},
            )
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("tencent login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                2,
                id,
                "tencent",
                verify_timeout=60,
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("tencent login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        except LoginPersistFailed as exc:
            login_logger.warning("tencent login state persist failed account_name={} error={}", id, exc)
            push_login_failed_status(status_queue, command_queue, str(exc))
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# 快手登录
async def get_ks_cookie(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()
    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()
    async with async_playwright() as playwright:
        options = get_login_browser_options(command_queue=command_queue, extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://cp.kuaishou.com")

        # 定位并点击“立即登录”按钮（类型为 link）
        await page.get_by_role("link", name="立即登录").click()
        await page.get_by_text("扫码登录").click()
        img_locator = page.get_by_role("img", name="qrcode")
        qr_data = await locator_to_data_url(img_locator)
        original_url = page.url
        login_logger.info("kuaishou qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
                success_validator=build_login_success_validator(4),
            )
            if login_result == "cancelled":
                login_logger.info("kuaishou login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("kuaishou login navigation detected account_name={}", id)
            push_structured_status(
                status_queue,
                command_queue,
                "running",
                {"message": "安全验证通过，正在测试并保存登录配置，此过程通常约 10-15 秒，偶遇网络或平台校验可能长达一分钟，请耐心等待..."},
            )
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("kuaishou login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                4,
                id,
                "kuaishou",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("kuaishou login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        except LoginPersistFailed as exc:
            login_logger.warning("kuaishou login state persist failed account_name={} error={}", id, exc)
            push_login_failed_status(status_queue, command_queue, str(exc))
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# 小红书登录
async def xiaohongshu_cookie_gen(id,status_queue, command_queue=None):
    url_changed_event = asyncio.Event()

    async def on_url_change():
        # 检查是否是主框架的变化
        if page.url != original_url:
            url_changed_event.set()

    async with async_playwright() as playwright:
        options = get_login_browser_options(command_queue=command_queue, extra_args=['--lang=en-GB'])
        browser = await playwright.chromium.launch(**options)
        # Setup context however you like.
        context = await browser.new_context()  # Pass any options
        context = await set_init_script(context)
        # Pause the page, and start recording manually.
        page = await context.new_page()
        await page.goto("https://creator.xiaohongshu.com/")
        await page.locator('img.css-wemwzq').click()

        img_locator = page.get_by_role("img").nth(2)
        qr_data = await locator_to_data_url(img_locator)
        original_url = page.url
        login_logger.info("xiaohongshu qr generated account_name={}", id)
        status_queue.put(qr_data)
        # 监听页面的 'framenavigated' 事件，只关注主框架的变化
        page.on('framenavigated',
                lambda frame: asyncio.create_task(on_url_change()) if frame == page.main_frame else None)

        try:
            login_result = await wait_for_login_result(
                page,
                original_url,
                url_changed_event,
                status_queue,
                command_queue,
                timeout=200,
                qr_locator=img_locator,
                qr_action_root=page,
                initial_qr_data=qr_data,
                success_validator=build_login_success_validator(1),
            )
            if login_result == "cancelled":
                login_logger.info("xiaohongshu login cancelled account_name={}", id)
                await page.close()
                await context.close()
                await browser.close()
                status_queue.put("CANCELLED")
                return None
            if not login_result:
                raise asyncio.TimeoutError
            login_logger.info("xiaohongshu login navigation detected account_name={}", id)
            push_structured_status(
                status_queue,
                command_queue,
                "running",
                {"message": "安全验证通过，正在测试并保存登录配置，此过程通常约 10-15 秒，偶遇网络或平台校验可能长达一分钟，请耐心等待..."},
            )
        except asyncio.TimeoutError:
            status_queue.put("500")
            login_logger.warning("xiaohongshu login timed out account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            return None
        try:
            saved_file = await persist_login_state_with_retry(
                context,
                1,
                id,
                "xiaohongshu",
                page=page,
                status_queue=status_queue,
                command_queue=command_queue,
            )
        except LoginCancelled:
            login_logger.info("xiaohongshu login cancelled while waiting cookie ready account_name={}", id)
            await page.close()
            await context.close()
            await browser.close()
            status_queue.put("CANCELLED")
            return None
        except LoginPersistFailed as exc:
            login_logger.warning("xiaohongshu login state persist failed account_name={} error={}", id, exc)
            push_login_failed_status(status_queue, command_queue, str(exc))
            await page.close()
            await context.close()
            await browser.close()
            return None
        await page.close()
        await context.close()
        await browser.close()
        status_queue.put("200")

# a = asyncio.run(xiaohongshu_cookie_gen(4,None))
# print(a)
