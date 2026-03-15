from pydantic import BaseModel, Field

from app.schemas.account import RemoteLoginSessionResponse
from app.schemas.device import DeviceResponse


class AgentHeartbeatRequest(BaseModel):
    device_code: str = Field(min_length=4, max_length=64)
    device_name: str | None = Field(default=None, max_length=120)
    agent_key: str = Field(min_length=4, max_length=255)
    local_ip: str | None = Field(default=None, max_length=64)
    public_ip: str | None = Field(default=None, max_length=64)
    runtime_payload: dict | None = None


class AgentHeartbeatResponse(BaseModel):
    device: DeviceResponse


class AgentLoginTaskResponse(RemoteLoginSessionResponse):
    pass


class AgentLoginSessionEventRequest(BaseModel):
    status: str = Field(min_length=1, max_length=32)
    message: str | None = Field(default=None, max_length=255)
    qr_data: str | None = None
    verification_payload: dict | None = None
