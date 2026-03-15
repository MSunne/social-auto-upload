from datetime import datetime

from pydantic import BaseModel, Field

from app.schemas.base import TimestampedResponse


class DeviceClaimRequest(BaseModel):
    device_code: str = Field(min_length=4, max_length=64)


class DeviceUpdateRequest(BaseModel):
    name: str | None = Field(default=None, min_length=1, max_length=120)
    default_reasoning_model: str | None = Field(default=None, max_length=120)
    is_enabled: bool | None = None


class DeviceResponse(TimestampedResponse):
    owner_user_id: str | None
    device_code: str
    name: str
    local_ip: str | None
    public_ip: str | None
    default_reasoning_model: str | None
    is_enabled: bool
    runtime_payload: dict | None
    last_seen_at: datetime | None
    notes: str | None

