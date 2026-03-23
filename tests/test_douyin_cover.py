import unittest
from unittest import mock

from uploader.douyin_uploader.main import DouYinVideo


class _FakePage:
    def __init__(self):
        self.wait_for_timeout = mock.AsyncMock()


class DouyinCoverTests(unittest.IsolatedAsyncioTestCase):
    async def test_handle_auto_video_cover_retries_until_platform_accepts_cover(self):
        page = _FakePage()
        video = DouYinVideo("标题", "/tmp/video.mp4", [], 0, "account.json")

        with mock.patch.object(
            video,
            "_cover_requirement_satisfied",
            new=mock.AsyncMock(side_effect=[False, False, False, True]),
        ), mock.patch.object(
            video,
            "_dismiss_vertical_cover_prompt",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            video,
            "_cover_warning_visible",
            new=mock.AsyncMock(side_effect=[True, True]),
        ), mock.patch.object(
            video,
            "_find_cover_modal",
            new=mock.AsyncMock(side_effect=[None, object()]),
        ), mock.patch.object(
            video,
            "_open_cover_picker",
            new=mock.AsyncMock(return_value=True),
        ) as open_mock, mock.patch.object(
            video,
            "_select_recommend_cover",
            new=mock.AsyncMock(return_value=True),
        ) as select_mock, mock.patch.object(
            video,
            "_confirm_cover_selection",
            new=mock.AsyncMock(return_value=True),
        ) as confirm_mock:
            result = await video.handle_auto_video_cover(page, max_attempts=2)

        self.assertTrue(result)
        open_mock.assert_awaited_once()
        self.assertEqual(select_mock.await_count, 2)
        self.assertEqual(confirm_mock.await_count, 2)

    async def test_handle_auto_video_cover_fails_after_max_attempts(self):
        page = _FakePage()
        video = DouYinVideo("标题", "/tmp/video.mp4", [], 0, "account.json")

        with mock.patch.object(
            video,
            "_cover_requirement_satisfied",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            video,
            "_dismiss_vertical_cover_prompt",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            video,
            "_cover_warning_visible",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            video,
            "_find_cover_modal",
            new=mock.AsyncMock(return_value=None),
        ), mock.patch.object(
            video,
            "_open_cover_picker",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            video,
            "_select_recommend_cover",
            new=mock.AsyncMock(return_value=False),
        ), mock.patch.object(
            video,
            "_confirm_cover_selection",
            new=mock.AsyncMock(return_value=False),
        ):
            result = await video.handle_auto_video_cover(page, max_attempts=2)

        self.assertFalse(result)

    async def test_handle_auto_video_cover_dismisses_vertical_cover_prompt(self):
        page = _FakePage()
        video = DouYinVideo("标题", "/tmp/video.mp4", [], 0, "account.json")

        with mock.patch.object(
            video,
            "_cover_requirement_satisfied",
            new=mock.AsyncMock(side_effect=[False, True]),
        ), mock.patch.object(
            video,
            "_dismiss_vertical_cover_prompt",
            new=mock.AsyncMock(side_effect=[False, True]),
        ) as dismiss_prompt_mock, mock.patch.object(
            video,
            "_cover_warning_visible",
            new=mock.AsyncMock(return_value=True),
        ), mock.patch.object(
            video,
            "_find_cover_modal",
            new=mock.AsyncMock(return_value=object()),
        ), mock.patch.object(
            video,
            "_select_recommend_cover",
            new=mock.AsyncMock(return_value=True),
        ) as select_mock, mock.patch.object(
            video,
            "_confirm_cover_selection",
            new=mock.AsyncMock(return_value=True),
        ) as confirm_mock:
            result = await video.handle_auto_video_cover(page, max_attempts=1)

        self.assertTrue(result)
        self.assertEqual(dismiss_prompt_mock.await_count, 2)
        select_mock.assert_awaited_once()
        confirm_mock.assert_awaited_once()
