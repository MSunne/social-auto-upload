from pydantic import BaseModel, Field

from app.schemas.base import TimestampedResponse


class ProductSkillCreateRequest(BaseModel):
    name: str = Field(min_length=1, max_length=120)
    description: str = Field(min_length=1)
    output_type: str = Field(min_length=1, max_length=32)
    model_name: str = Field(min_length=1, max_length=120)
    prompt_template: str | None = None
    reference_payload: dict | None = None
    is_enabled: bool = True


class ProductSkillUpdateRequest(BaseModel):
    name: str | None = Field(default=None, min_length=1, max_length=120)
    description: str | None = None
    output_type: str | None = Field(default=None, min_length=1, max_length=32)
    model_name: str | None = Field(default=None, min_length=1, max_length=120)
    prompt_template: str | None = None
    reference_payload: dict | None = None
    is_enabled: bool | None = None


class ProductSkillResponse(TimestampedResponse):
    owner_user_id: str
    name: str
    description: str
    output_type: str
    model_name: str
    prompt_template: str | None
    reference_payload: dict | None
    is_enabled: bool

