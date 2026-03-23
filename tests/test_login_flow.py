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
                "_retryCount": 8,
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
