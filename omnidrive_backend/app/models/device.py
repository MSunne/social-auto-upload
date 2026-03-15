from datetime import datetime

from sqlalchemy import Boolean, DateTime, ForeignKey, JSON, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.models.base import Base, TimestampedUUIDMixin


class Device(TimestampedUUIDMixin, Base):
    __tablename__ = "devices"

    owner_user_id: Mapped[str | None] = mapped_column(ForeignKey("users.id"), index=True, nullable=True)
    device_code: Mapped[str] = mapped_column(String(64), unique=True, index=True, nullable=False)
    agent_key: Mapped[str | None] = mapped_column(String(255), nullable=True)
    name: Mapped[str] = mapped_column(String(120), nullable=False)
    local_ip: Mapped[str | None] = mapped_column(String(64), nullable=True)
    public_ip: Mapped[str | None] = mapped_column(String(64), nullable=True)
    default_reasoning_model: Mapped[str | None] = mapped_column(String(120), nullable=True)
    is_enabled: Mapped[bool] = mapped_column(Boolean, default=False, nullable=False)
    runtime_payload: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    last_seen_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    owner = relationship("User")

