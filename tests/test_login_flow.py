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

    def is_closed(self):
        return self.closed


class _FakeContext:
    def __init__(self, storage_state):
        self._storage_state = storage_state

    async def storage_state(self):
        return self._storage_state


class LoginFlowTests(unittest.IsolatedAsyncioTestCase):
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

    async def test_fill_text_and_submit_prefers_submit_button_before_keyboard_enter(self):
        page = _FakePage()
        input_locator = mock.Mock()
        input_locator.press = mock.AsyncMock()

        with mock.patch.object(
            login_module,
            "find_first_editable_input",
            new=mock.AsyncMock(return_value=input_locator),
        ), mock.patch.object(
            login_module,
            "fill_input_like_user",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            login_module,
            "click_submit_action",
            new=mock.AsyncMock(return_value=True),
        ) as submit_mock:
            result = await login_module.apply_remote_action(
                page,
                {
                    "actionType": "fill_text_and_submit",
                    "payload": {"text": "123456"},
                },
            )

        self.assertTrue(result)
        submit_mock.assert_awaited_once_with(page)
        input_locator.press.assert_not_awaited()
        page.keyboard.press.assert_not_awaited()

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

    async def test_submit_verification_action_falls_back_to_enter_when_click_has_no_effect(self):
        page = _FakePage()
        input_locator = mock.Mock()
        input_locator.click = mock.AsyncMock()
        input_locator.press = mock.AsyncMock()

        signatures = iter(["same", "same", "same", "changed"])

        async def fake_snapshot(_page):
            return next(signatures)

        with mock.patch.object(
            login_module,
            "get_verification_signature_snapshot",
            new=mock.AsyncMock(side_effect=fake_snapshot),
        ), mock.patch.object(
            login_module,
            "click_submit_action",
            new=mock.AsyncMock(return_value=True),
        ) as click_mock:
            result = await login_module.submit_verification_action(page, input_locator=input_locator)

        self.assertTrue(result)
        click_mock.assert_awaited_once_with(page)
        input_locator.press.assert_awaited_once_with("Enter")

    async def test_submit_verification_action_returns_false_when_no_strategy_dispatches(self):
        page = _FakePage()
        input_locator = mock.Mock()
        input_locator.click = mock.AsyncMock(side_effect=RuntimeError("no focus"))
        input_locator.press = mock.AsyncMock(side_effect=RuntimeError("no enter"))
        page.keyboard.press = mock.AsyncMock(side_effect=RuntimeError("blocked"))

        with mock.patch.object(
            login_module,
            "get_verification_signature_snapshot",
            new=mock.AsyncMock(return_value="same"),
        ), mock.patch.object(
            login_module,
            "click_submit_action",
            new=mock.AsyncMock(return_value=False),
        ):
            result = await login_module.submit_verification_action(page, input_locator=input_locator)

        self.assertFalse(result)

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

    async def test_select_option_falls_back_to_resend_action_for_code_requests(self):
        page = _FakePage()

        async def fake_click_verification_option(_page, text):
            return text == "重新发送"

        with mock.patch.object(
            login_module,
            "click_verification_option",
            new=mock.AsyncMock(side_effect=fake_click_verification_option),
        ) as click_mock:
            result = await login_module.apply_remote_action(
                page,
                {
                    "actionType": "select_option",
                    "payload": {"optionText": "接收短信验证码"},
                },
            )

        self.assertTrue(result)
        attempted_texts = [call.args[1] for call in click_mock.await_args_list]
        self.assertIn("重新发送", attempted_texts)

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

    async def test_drain_remote_actions_requeues_failed_action_for_retry(self):
        page = _FakePage()
        command_queue = Queue()
        command_queue.put(
            {
                "actionType": "fill_text_and_submit",
                "payload": {"text": "123456"},
            }
        )

        with mock.patch.object(
            login_module,
            "apply_remote_action",
            new=mock.AsyncMock(return_value=False),
        ):
            handled = await login_module.drain_remote_actions(page, command_queue)

        self.assertFalse(handled)
        retried = command_queue.get_nowait()
        self.assertEqual(retried["actionType"], "fill_text_and_submit")
        self.assertEqual(retried["_retryCount"], 1)

    async def test_drain_remote_actions_drops_action_after_retry_limit(self):
        page = _FakePage()
        command_queue = Queue()
        command_queue.put(
            {
                "actionType": "fill_text_and_submit",
                "payload": {"text": "123456"},
                "_retryCount": login_module.REMOTE_ACTION_MAX_RETRIES,
            }
        )

        with mock.patch.object(
            login_module,
            "apply_remote_action",
            new=mock.AsyncMock(return_value=False),
        ):
            handled = await login_module.drain_remote_actions(page, command_queue)

        self.assertFalse(handled)
        self.assertTrue(command_queue.empty())

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
