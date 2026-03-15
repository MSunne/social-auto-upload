from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.models.base import Base, TimestampedUUIDMixin


class PlatformAccount(TimestampedUUIDMixin, Base):
    __tablename__ = "platform_accounts"

    device_id: Mapped[str] = mapped_column(ForeignKey("devices.id"), index=True, nullable=False)
    platform: Mapped[str] = mapped_column(String(32), index=True, nullable=False)
    account_name: Mapped[str] = mapped_column(String(120), nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="verifying", nullable=False)
    last_message: Mapped[str | None] = mapped_column(String(255), nullable=True)
    last_authenticated_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    device = relationship("Device")

