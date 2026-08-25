from datetime import datetime

from sqlalchemy import DateTime, ForeignKey, Integer, String
from sqlalchemy.orm import Mapped, mapped_column, relationship

from backend.app.common.ids import new_id
from backend.app.common.models import Base, TimestampMixin
from backend.app.common.types import JSONType, JSONValue
from backend.app.devices.policies import default_telemetry_policy


class Device(TimestampMixin, Base):
    __tablename__ = "devices"

    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=new_id)
    vehicle_id: Mapped[str] = mapped_column(
        ForeignKey("vehicles.id", ondelete="CASCADE"), index=True
    )
    name: Mapped[str] = mapped_column(String(120))
    credential_hash: Mapped[str] = mapped_column(String(64), unique=True, index=True)
    credential_version: Mapped[int] = mapped_column(Integer, default=1)
    agent_version: Mapped[str | None] = mapped_column(String(50))
    hostname: Mapped[str | None] = mapped_column(String(255))
    hardware: Mapped[JSONValue] = mapped_column(JSONType, default=dict)
    last_seen_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), index=True)
    last_config_sync_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
    config_version: Mapped[int] = mapped_column(Integer, default=1)
    telemetry_policy: Mapped[JSONValue] = mapped_column(JSONType, default=default_telemetry_policy)
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))

    vehicle = relationship("Vehicle", back_populates="devices")


class EnrollmentToken(Base):
    __tablename__ = "device_enrollment_tokens"

    id: Mapped[str] = mapped_column(String(36), primary_key=True, default=new_id)
    token_hash: Mapped[str] = mapped_column(String(64), unique=True, index=True)
    vehicle_id: Mapped[str] = mapped_column(
        ForeignKey("vehicles.id", ondelete="CASCADE"), index=True
    )
    intended_name: Mapped[str] = mapped_column(String(120))
    telemetry_policy: Mapped[JSONValue] = mapped_column(JSONType, default=default_telemetry_policy)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True))
    expires_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), index=True)
    used_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True))
