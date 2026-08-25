from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, model_validator

from backend.app.devices.policies import DEFAULT_TELEMETRY_POLICY


class TelemetryPolicy(BaseModel):
    sampling_seconds: int = Field(
        default=DEFAULT_TELEMETRY_POLICY["sampling_seconds"], ge=1, le=86400
    )
    upload_seconds: int = Field(default=DEFAULT_TELEMETRY_POLICY["upload_seconds"], ge=1, le=86400)
    parked_sampling_seconds: int = Field(
        default=DEFAULT_TELEMETRY_POLICY["parked_sampling_seconds"], ge=1, le=86400
    )
    parked_upload_seconds: int = Field(
        default=DEFAULT_TELEMETRY_POLICY["parked_upload_seconds"], ge=1, le=86400
    )

    @model_validator(mode="after")
    def matching_sample_and_upload_intervals(self) -> "TelemetryPolicy":
        if self.upload_seconds != self.sampling_seconds:
            raise ValueError("driving sampling and upload intervals must match")
        if self.parked_upload_seconds != self.parked_sampling_seconds:
            raise ValueError("parked sampling and upload intervals must match")
        return self


class EnrollmentCreate(BaseModel):
    name: str = Field(default="Vehicle tracker", min_length=1, max_length=120)
    ttl_minutes: int = Field(default=30, ge=5, le=1440)
    telemetry_policy: TelemetryPolicy = Field(default_factory=TelemetryPolicy)


class AgentSetupStep(BaseModel):
    kind: Literal["command", "value", "link", "manual"]
    text: str = ""
    command: str = ""
    value: str = ""
    url: str = ""


class AgentImplementation(BaseModel):
    id: str
    name: str
    hardware: str
    setup_kind: Literal["command", "guided"]
    docs_url: str = ""


class AgentInstallation(AgentImplementation):
    setup_steps: list[AgentSetupStep]


class EnrollmentCreated(BaseModel):
    token: str
    expires_at: datetime
    server_url: str
    server_version: str
    implementations: list[AgentInstallation]


class EnrollRequest(BaseModel):
    token: str = Field(min_length=20, max_length=200)
    agent_version: str = Field(min_length=1, max_length=50)
    hostname: str = Field(default="", max_length=255)
    hardware: dict[str, object] = Field(default_factory=dict)


class DeviceConfig(BaseModel):
    version: int
    sampling: dict[str, int]
    upload: dict[str, int]
    vehicle_profile: str | None
    vehicle_profile_definition: dict[str, object] | None = None


class EnrollResponse(BaseModel):
    device_id: str
    vehicle_id: str
    credential: str
    config: DeviceConfig


class RotateCredentialResponse(BaseModel):
    credential: str
    credential_version: int


class DeviceResponse(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    vehicle_id: str
    name: str
    credential_version: int
    agent_version: str | None
    version_compatibility: Literal["compatible", "warning", "incompatible", "unknown"]
    hostname: str | None
    hardware: dict[str, object]
    online: bool
    last_seen_at: datetime | None
    last_config_sync_at: datetime | None
    config_version: int
    telemetry_policy: TelemetryPolicy
    revoked_at: datetime | None
    created_at: datetime
