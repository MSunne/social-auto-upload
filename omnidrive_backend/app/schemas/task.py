from datetime import datetime

from app.schemas.base import TimestampedResponse


class PublishTaskResponse(TimestampedResponse):
    device_id: str
    account_id: str | None
    skill_id: str | None
    platform: str
    account_name: str
    title: str
    content_text: str | None
    media_payload: dict | None
    status: str
    message: str | None
    verification_payload: dict | None
    run_at: datetime | None
    finished_at: datetime | None

