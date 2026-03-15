from datetime import datetime

from pydantic import BaseModel, Field

from app.schemas.base import TimestampedResponse


class AccountResponse(TimestampedResponse):
    device_id: str
    platform: str
    account_name: str
    status: str
    last_message: str | None
    last_authenticated_at: datetime | None


class RemoteLoginSessionCreateRequest(BaseModel):
    device_id: str
    platform: str = Field(min_length=1, max_length=32)
    account_name: str = Field(min_length=1, max_length=120)


class RemoteLoginSessionResponse(TimestampedResponse):
    device_id: str
    user_id: str
    platform: str
    account_name: str
    status: str
    qr_data: str | None
    verification_payload: dict | None
    message: str | None

