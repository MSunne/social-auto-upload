import threading
import unittest
from queue import Queue
from unittest import mock

from myUtils import login as login_module


class _FakeKeyboard:
    def __init__(self):
        self.press = mock.AsyncMock()


class _FakePage:
    def __init__(self, url="https://creator.test/login"):
        self.url = url
        self.keyboard = _FakeKeyboard()
        self.closed = False
        self.main_frame = object()
        self.goto = mock.AsyncMock(side_effect=self._goto)

    async def _goto(self, url, *args, **kwargs):
        self.url = url

    def is_closed(self):
        return self.closed

    def on(self, *_args, **_kwargs):
        return None


class _FakeContext:
    def __init__(self, storage_state, page=None):
        self._storage_state = storage_state
        self._page = page or _FakePage()
        self.pages = [self._page]
        self.new_page = mock.AsyncMock(return_value=self._page)
        self.close = mock.AsyncMock()

    async def storage_state(self):
        return self._storage_state


class _FakeBrowser:
    def __init__(self, context):
        self._context = context
        self.new_context = mock.AsyncMock(return_value=context)
        self.close = mock.AsyncMock()
        self._connected = True

    def is_connected(self):
        return self._connected


class _FakePlaywright:
    def __init__(self, browser):
        self.chromium = mock.Mock()
        self.chromium.launch = mock.AsyncMock(return_value=browser)


class _FakePlaywrightContextManager:
    def __init__(self, playwright):
        self._playwright = playwright

    async def __aenter__(self):
        return self._playwright

    async def __aexit__(self, exc_type, exc, tb):
        return False


class LoginFlowTests(unittest.IsolatedAsyncioTestCase):
    def test_push_structured_status_supports_local_queue_without_command_channel(self):
        status_queue = Queue()

        login_module.push_structured_status(
            status_queue,
            command_queue=None,
            event_type="login_failed",
            payload={"message": "cookie 校验失败"},
        )

        self.assertEqual(
            status_queue.get_nowait(),
            {
                "type": "login_failed",
                "payload": {"message": "cookie 校验失败"},
            },
        )

    def test_get_login_wait_timeout_uses_long_timeout_for_local_login(self):
        self.assertEqual(login_module.get_login_wait_timeout(command_queue=object()), 200)
        self.assertGreater(login_module.get_login_wait_timeout(command_queue=None), 200)

    async def test_finalize_successful_login_keeps_local_browser_open_until_user_closes_it(self):
        status_queue = Queue()
        page = mock.AsyncMock()
        context = mock.AsyncMock()
        browser = mock.AsyncMock()

        with mock.patch.object(
            login_module,
            "keep_local_login_browser_open",
            new=mock.AsyncMock(),
        ) as keep_open_mock:
            await login_module.finalize_successful_login(
                status_queue,
                page,
                context,
                browser,
                keep_browser_open=True,
            )

        self.assertEqual(status_queue.get_nowait(), "200")
        keep_open_mock.assert_awaited_once_with(context, browser)
        page.close.assert_not_awaited()
        context.close.assert_not_awaited()
        browser.close.assert_not_awaited()

    def test_dedupe_verification_option_labels_prefers_specific_choices(self):
        labels = [
            "接收短信验证码",
            "短信验证码",
            "验证登录密码",
            "登录密码",
            "继续",
            "接收短信验证码",
        ]

        result = login_module.dedupe_verification_option_labels(labels)

        self.assertEqual(result, ["接收短信验证码", "验证登录密码"])

    def test_dedupe_verification_option_labels_collapses_dynamic_resend_countdowns(self):
        labels = [
            "38s后重新发送",
            "37s后重新发送",
            "重新发送验证码",
        ]

        result = login_module.dedupe_verification_option_labels(labels)

        self.assertEqual(result, ["重新发送验证码"])

    def test_refine_verification_option_labels_prefers_current_code_step_actions(self):
        labels = [
            "登录密码",
            "获取验证码",
            "接收短信验证码",
            "发送验证码",
        ]

        result = login_module.refine_verification_option_labels(
            labels,
            title="登录验证",
            input_hints=["请输入验证码"],
        )

        self.assertEqual(result, ["接收短信验证码"])

    def test_is_strong_verification_challenge_rejects_weak_generic_qr_signal(self):
        challenge = {
            "payload": {
                "title": "需要额外验证",
                "options": ["获取验证码"],
                "inputHints": ["请输入手机号 / 请输入验证码"],
                "supportsTextInput": True,
            }
        }

        self.assertFalse(login_module.is_strong_verification_challenge(challenge))

    def test_is_strong_verification_challenge_accepts_specific_sms_flow(self):
        challenge = {
            "payload": {
                "title": "接收短信验证码",
                "options": ["接收短信验证码"],
                "inputHints": ["请输入验证码"],
                "supportsTextInput": True,
            }
        }

        self.assertTrue(login_module.is_strong_verification_challenge(challenge))

    def test_score_action_label_match_rejects_generic_verification_title_for_submit(self):
        self.assertLess(login_module.score_action_label_match("接收短信验证码", "验证"), 0)

    def test_score_action_label_match_accepts_compact_submit_button_labels(self):
        self.assertGreater(login_module.score_action_label_match("验证", "验证"), 0)
        self.assertGreater(login_module.score_action_label_match("立即验证", "验证"), 0)

    async def test_click_verification_option_prefers_interactive_action_over_generic_text(self):
        page = _FakePage()
        target = mock.Mock()

        with mock.patch.object(
            login_module,
            "get_verification_targets",
            new=mock.AsyncMock(return_value=[target]),
        ), mock.patch.object(
            login_module,
            "click_best_matching_interactive_action",
            new=mock.AsyncMock(return_value=True),
        ) as interactive_mock, mock.patch.object(
            login_module,
            "click_visible_option",
            new=mock.AsyncMock(return_value=False),
        ) as exact_mock, mock.patch.object(
            login_module,
            "click_visible_partial_option",
            new=mock.AsyncMock(return_value=False),
        ) as partial_mock:
            result = await login_module.click_verification_option(page, "接收短信验证码")

        self.assertTrue(result)
        interactive_mock.assert_awaited_once_with(target, ["接收短信验证码"])
        exact_mock.assert_not_awaited()
        partial_mock.assert_not_awaited()

    async def test_open_platform_backend_session_reuses_valid_cookie_and_marks_account_healthy(self):
        page = _FakePage(url="https://creator.test/manage")
        context = _FakeContext({"cookies": [{"name": "sid"}], "origins": []}, page=page)
        browser = _FakeBrowser(context)
        playwright = _FakePlaywright(browser)

        with mock.patch.object(
            login_module,
            "async_playwright",
            return_value=_FakePlaywrightContextManager(playwright),
        ), mock.patch.object(
            login_module,
            "resolve_account_storage_state",
            return_value={"cookies": [{"name": "sid"}], "origins": []},
        ), mock.patch.object(
            login_module,
            "set_init_script",
            new=mock.AsyncMock(side_effect=lambda ctx: ctx),
        ), mock.patch.object(
            login_module,
            "validate_active_page_detail",
            new=mock.AsyncMock(return_value={"ok": True, "message": "ok"}),
        ), mock.patch.object(
            login_module,
            "dismiss_platform_popups",
            new=mock.AsyncMock(),
        ), mock.patch.object(
            login_module,
            "persist_account_storage_state",
            new=mock.AsyncMock(return_value={"cookies": [{"name": "sid"}], "origins": []}),
        ) as persist_mock, mock.patch.object(
            login_module,
            "update_account_runtime_status",
        ) as update_status_mock:
            await login_module.open_platform_backend_session(3, "测试抖音账号", 7, keep_window_open=False)

        self.assertEqual(page.goto.await_count, 1)
        persist_mock.assert_awaited()
        update_status_mock.assert_called_with(7, 1, None)
        browser.close.assert_awaited()

    async def test_open_platform_backend_session_persists_cookie_after_qr_login(self):
        page = _FakePage(url="https://creator.test/manage")
        context = _FakeContext({"cookies": [{"name": "sid"}], "origins": []}, page=page)
        browser = _FakeBrowser(context)
        playwright = _FakePlaywright(browser)
        qr_locator = mock.Mock()

        with mock.patch.object(
            login_module,
            "async_playwright",
            return_value=_FakePlaywrightContextManager(playwright),
        ), mock.patch.object(
            login_module,
            "resolve_account_storage_state",
            return_value={"cookies": [{"name": "sid"}], "origins": []},
        ), mock.patch.object(
            login_module,
            "set_init_script",
            new=mock.AsyncMock(side_effect=lambda ctx: ctx),
        ), mock.patch.object(
            login_module,
            "validate_active_page_detail",
            new=mock.AsyncMock(return_value={"ok": False, "message": "需要重新扫码登录"}),
        ), mock.patch.object(
            login_module,
            "prepare_backend_login_session",
            new=mock.AsyncMock(return_value=("https://creator.test/login", qr_locator, page)),
        ), mock.patch.object(
            login_module,
            "wait_for_login_result",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            login_module,
            "persist_login_state_with_retry",
            new=mock.AsyncMock(return_value="douyin.json"),
        ) as persist_login_mock, mock.patch.object(
            login_module,
            "dismiss_platform_popups",
            new=mock.AsyncMock(),
        ), mock.patch.object(
            login_module,
            "persist_account_storage_state",
            new=mock.AsyncMock(return_value={"cookies": [{"name": "sid"}], "origins": []}),
        ) as persist_state_mock, mock.patch.object(
            login_module,
            "update_account_runtime_status",
        ) as update_status_mock:
            await login_module.open_platform_backend_session(3, "测试抖音账号", 8, keep_window_open=False)

        persist_login_mock.assert_awaited_once()
        self.assertGreaterEqual(persist_state_mock.await_count, 1)
        self.assertEqual(page.goto.await_count, 2)
        self.assertEqual(update_status_mock.call_args_list[-1], mock.call(8, 1, None))

    async def test_open_platform_backend_session_keeps_existing_cookie_when_login_not_completed(self):
        page = _FakePage(url="https://creator.test/manage")
        context = _FakeContext({"cookies": [{"name": "sid"}], "origins": []}, page=page)
        browser = _FakeBrowser(context)
        playwright = _FakePlaywright(browser)
        qr_locator = mock.Mock()

        with mock.patch.object(
            login_module,
            "async_playwright",
            return_value=_FakePlaywrightContextManager(playwright),
        ), mock.patch.object(
            login_module,
            "resolve_account_storage_state",
            return_value={"cookies": [{"name": "sid"}], "origins": []},
        ), mock.patch.object(
            login_module,
            "set_init_script",
            new=mock.AsyncMock(side_effect=lambda ctx: ctx),
        ), mock.patch.object(
            login_module,
            "validate_active_page_detail",
            new=mock.AsyncMock(return_value={"ok": False, "message": "需要重新扫码登录"}),
        ), mock.patch.object(
            login_module,
            "prepare_backend_login_session",
            new=mock.AsyncMock(return_value=("https://creator.test/login", qr_locator, page)),
        ), mock.patch.object(
            login_module,
            "wait_for_login_result",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "persist_login_state_with_retry",
            new=mock.AsyncMock(),
        ) as persist_login_mock, mock.patch.object(
            login_module,
            "dismiss_platform_popups",
            new=mock.AsyncMock(),
        ), mock.patch.object(
            login_module,
            "persist_account_storage_state",
            new=mock.AsyncMock(),
        ) as persist_state_mock, mock.patch.object(
            login_module,
            "update_account_runtime_status",
        ) as update_status_mock:
            await login_module.open_platform_backend_session(4, "测试快手账号", 9, keep_window_open=False)

        persist_login_mock.assert_not_awaited()
        persist_state_mock.assert_not_awaited()
        self.assertEqual(update_status_mock.call_args_list, [mock.call(9, 0, "需要重新扫码登录")])

    async def test_click_submit_action_allows_exact_short_submit_label_without_partial_fallback(self):
        page = _FakePage()
        target = mock.Mock()

        with mock.patch.object(
            login_module,
            "get_verification_targets",
            new=mock.AsyncMock(return_value=[target]),
        ), mock.patch.object(
            login_module,
            "click_best_matching_interactive_action",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "click_visible_option",
            new=mock.AsyncMock(side_effect=lambda _target, text: text == "验证"),
        ) as exact_mock, mock.patch.object(
            login_module,
            "click_visible_partial_option",
            new=mock.AsyncMock(return_value=False),
        ) as partial_mock:
            result = await login_module.click_submit_action(page)

        self.assertTrue(result)
        attempted_exact = [call.args[1] for call in exact_mock.await_args_list]
        attempted_partial = [call.args[1][0] for call in partial_mock.await_args_list]
        self.assertIn("验证", attempted_exact)
        self.assertNotIn("验证", attempted_partial)

    async def test_detect_verification_challenge_uses_dynamic_status_lines_in_message_and_signature(self):
        page = _FakePage(url="https://creator.test/verify")

        with mock.patch.object(
            login_module,
            "get_verification_anchor",
            new=mock.AsyncMock(return_value=("接收短信验证码", ["重新发送"], mock.Mock())),
        ), mock.patch.object(
            login_module,
            "get_verification_search_target",
            new=mock.AsyncMock(return_value=page),
        ), mock.patch.object(
            login_module,
            "collect_visible_option_texts",
            new=mock.AsyncMock(return_value=["重新发送"]),
        ), mock.patch.object(
            login_module,
            "get_visible_verification_input_hints",
            new=mock.AsyncMock(return_value=["请输入验证码"]),
        ), mock.patch.object(
            login_module,
            "get_visible_verification_status_texts",
            new=mock.AsyncMock(return_value=["验证码错误，请重新输入", "14s后重新发送"]),
        ), mock.patch.object(
            login_module,
            "has_visible_verification_submit",
            new=mock.AsyncMock(return_value=True),
        ):
            challenge = await login_module.detect_verification_challenge(page)

        self.assertEqual(challenge["payload"]["message"], "验证码错误，请重新输入；14s后重新发送")
        self.assertIn("验证码错误，请重新输入；14s后重新发送", challenge["signature"])

    async def test_wait_for_login_result_uses_success_validator_after_verification(self):
        page = _FakePage()
        url_changed_event = threading.Event()
        challenge = {
            "signature": "challenge-1",
            "payload": {
                "title": "短信验证",
                "message": "请输入验证码",
            },
        }
        detect_sequence = [challenge, None, None]

        async def fake_detect_verification(_page):
            if detect_sequence:
                return detect_sequence.pop(0)
            return None

        async def immediate_sleep(_seconds):
            return None

        success_validator = mock.AsyncMock(side_effect=[False, True])

        with mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(side_effect=fake_detect_verification),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ):
            result = await login_module.wait_for_login_result(
                page,
                original_url=page.url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=1,
                verification_settle_seconds=0,
                success_validator=success_validator,
                success_check_interval=0,
            )

        self.assertTrue(result)
        self.assertGreaterEqual(success_validator.await_count, 2)

    async def test_wait_for_login_result_detects_verification_even_while_qr_is_visible(self):
        page = _FakePage()
        url_changed_event = threading.Event()
        challenge = {
            "signature": "challenge-qr-visible",
            "payload": {
                "title": "短信验证",
                "message": "请输入验证码",
            },
        }
        detect_sequence = [challenge, None, None]

        async def fake_detect_verification(_page):
            if detect_sequence:
                return detect_sequence.pop(0)
            return None

        async def immediate_sleep(_seconds):
            return None

        success_validator = mock.AsyncMock(side_effect=[False, True])

        with mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(side_effect=fake_detect_verification),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ):
            result = await login_module.wait_for_login_result(
                page,
                original_url=page.url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=1,
                qr_locator=mock.Mock(),
                verification_settle_seconds=0,
                success_validator=success_validator,
                success_check_interval=0,
            )

        self.assertTrue(result)
        self.assertGreaterEqual(success_validator.await_count, 2)

    async def test_wait_for_login_result_ignores_weak_verification_signal_while_waiting_scan(self):
        page = _FakePage()
        url_changed_event = threading.Event()
        challenge = {
            "signature": "weak-challenge",
            "payload": {
                "title": "需要额外验证",
                "options": ["获取验证码"],
                "inputHints": ["请输入手机号 / 请输入验证码"],
                "supportsTextInput": True,
            },
        }

        async def immediate_sleep(_seconds):
            return None

        with mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(return_value=challenge),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ), mock.patch.object(
            login_module,
            "push_structured_status",
            new=mock.Mock(),
        ) as push_status_mock:
            result = await login_module.wait_for_login_result(
                page,
                original_url=page.url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=0,
                qr_locator=mock.Mock(),
            )

        self.assertFalse(result)
        push_status_mock.assert_not_called()

    async def test_wait_for_login_result_does_not_finish_when_qr_only_hides_for_local_login(self):
        page = _FakePage()
        url_changed_event = threading.Event()

        class _FakeLoop:
            def __init__(self):
                self._value = -1

            def time(self):
                self._value += 1
                return self._value

        async def immediate_sleep(_seconds):
            return None

        with mock.patch.object(
            login_module.asyncio,
            "get_running_loop",
            return_value=_FakeLoop(),
        ), mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(return_value=None),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ):
            result = await login_module.wait_for_login_result(
                page,
                original_url=page.url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=4,
                qr_locator=mock.Mock(),
                allow_qr_hidden_success=False,
            )

        self.assertFalse(result)

    async def test_wait_for_login_result_does_not_finish_on_navigation_before_verification_completes(self):
        page = _FakePage(url="https://creator.test/verify")
        original_url = "https://creator.test/login"
        url_changed_event = threading.Event()
        url_changed_event.set()
        challenge = {
            "signature": "challenge-1",
            "payload": {
                "title": "短信验证",
                "message": "请输入验证码",
            },
        }
        detect_sequence = [challenge, None, None]

        async def fake_detect_verification(_page):
            if detect_sequence:
                return detect_sequence.pop(0)
            return None

        async def immediate_sleep(_seconds):
            return None

        success_validator = mock.AsyncMock(side_effect=[False, True])

        with mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(side_effect=fake_detect_verification),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ):
            result = await login_module.wait_for_login_result(
                page,
                original_url=original_url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=1,
                verification_settle_seconds=0,
                success_validator=success_validator,
                success_check_interval=0,
            )

        self.assertTrue(result)
        self.assertGreaterEqual(success_validator.await_count, 2)

    async def test_persist_login_state_with_retry_continues_after_page_closed(self):
        page = _FakePage(url="https://creator.test/success")
        page.closed = True
        context = _FakeContext({"cookies": [{"name": "sid", "value": "ok"}], "origins": []})

        with mock.patch.object(
            login_module.uuid,
            "uuid1",
            return_value="persist-ok",
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "check_cookie_detail",
            new=mock.AsyncMock(return_value={"ok": True, "message": "cookie 有效"}),
        ), mock.patch.object(
            login_module,
            "save_login_account",
            new=mock.Mock(return_value="saved.json"),
        ) as save_mock:
            saved_file = await login_module.persist_login_state_with_retry(
                context,
                3,
                "测试账号",
                "douyin",
                verify_timeout=1,
                verify_interval=0,
                page=page,
            )

        self.assertEqual(saved_file, "persist-ok.json")
        save_mock.assert_called_once_with(3, "测试账号", "persist-ok.json", 1, storage_state=context._storage_state)

    async def test_wait_for_login_result_retries_transient_navigation_errors(self):
        page = _FakePage(url="https://creator.test/verify")
        original_url = "https://creator.test/login"
        url_changed_event = threading.Event()
        url_changed_event.set()
        challenge = {
            "signature": "challenge-1",
            "payload": {
                "title": "短信验证",
                "message": "请输入验证码",
            },
        }
        detect_sequence = [
            challenge,
            RuntimeError("Locator.count: Execution context was destroyed, most likely because of a navigation"),
            None,
        ]

        async def fake_detect_verification(_page):
            if detect_sequence:
                next_item = detect_sequence.pop(0)
                if isinstance(next_item, Exception):
                    raise next_item
                return next_item
            return None

        async def immediate_sleep(_seconds):
            return None

        success_validator = mock.AsyncMock(return_value=True)

        with mock.patch.object(
            login_module,
            "sync_login_qr_state",
            new=mock.AsyncMock(return_value={"isExpired": False, "isScanned": False}),
        ), mock.patch.object(
            login_module,
            "is_locator_visible",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module,
            "detect_verification_challenge",
            new=mock.AsyncMock(side_effect=fake_detect_verification),
        ), mock.patch.object(
            login_module,
            "drain_remote_actions",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            login_module.asyncio,
            "sleep",
            new=immediate_sleep,
        ):
            result = await login_module.wait_for_login_result(
                page,
                original_url=original_url,
                url_changed_event=url_changed_event,
                status_queue=None,
                command_queue=None,
                timeout=1,
                verification_settle_seconds=0,
                success_validator=success_validator,
                success_check_interval=0,
            )

        self.assertTrue(result)
