from sqlalchemy import ForeignKey, JSON, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.models.base import Base, TimestampedUUIDMixin


class LoginSession(TimestampedUUIDMixin, Base):
    __tablename__ = "login_sessions"

    device_id: Mapped[str] = mapped_column(ForeignKey("devices.id"), index=True, nullable=False)
    user_id: Mapped[str] = mapped_column(ForeignKey("users.id"), index=True, nullable=False)
    platform: Mapped[str] = mapped_column(String(32), index=True, nullable=False)
    account_name: Mapped[str] = mapped_column(String(120), nullable=False)
    status: Mapped[str] = mapped_column(String(32), default="pending", nullable=False)
    qr_data: Mapped[str | None] = mapped_column(Text, nullable=True)
    verification_payload: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    message: Mapped[str | None] = mapped_column(String(255), nullable=True)

    device = relationship("Device")
    user = relationship("User")

