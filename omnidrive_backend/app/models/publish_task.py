from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, JSON, String, Text
from sqlalchemy.orm import Mapped, mapped_column, relationship

from app.models.base import Base, TimestampedUUIDMixin


class PublishTask(TimestampedUUIDMixin, Base):
    __tablename__ = "publish_tasks"

    device_id: Mapped[str] = mapped_column(ForeignKey("devices.id"), index=True, nullable=False)
    account_id: Mapped[str | None] = mapped_column(ForeignKey("platform_accounts.id"), index=True, nullable=True)
    skill_id: Mapped[str | None] = mapped_column(ForeignKey("product_skills.id"), index=True, nullable=True)
    platform: Mapped[str] = mapped_column(String(32), index=True, nullable=False)
    account_name: Mapped[str] = mapped_column(String(120), nullable=False)
    title: Mapped[str] = mapped_column(String(200), nullable=False)
    content_text: Mapped[str | None] = mapped_column(Text, nullable=True)
    media_payload: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    status: Mapped[str] = mapped_column(String(32), default="pending", nullable=False)
    message: Mapped[str | None] = mapped_column(String(255), nullable=True)
    verification_payload: Mapped[dict | None] = mapped_column(JSON, nullable=True)
    run_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    finished_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)

    device = relationship("Device")
    account = relationship("PlatformAccount")
    skill = relationship("ProductSkill")

