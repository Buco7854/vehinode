from datetime import timedelta

from sqlalchemy import select
from sqlalchemy.orm import Session

from backend.app.auth.security import hash_token, new_opaque_token
from backend.app.branding import APP_VERSION
from backend.app.common.time import as_utc, utcnow
from backend.app.devices.models import Device, EnrollmentToken
from backend.app.devices.protocol import version_compatibility
from backend.app.devices.schemas import (
    DeviceConfig,
    EnrollRequest,
    EnrollResponse,
    TelemetryPolicy,
)
from backend.app.vehicle_profiles.services import profile_definition
from backend.app.vehicles.models import Vehicle


class EnrollmentError(Exception):
    pass


def device_config(db: Session, device: Device, vehicle: Vehicle) -> DeviceConfig:
    policy = TelemetryPolicy.model_validate(device.telemetry_policy)
    return DeviceConfig(
        version=device.config_version,
        sampling={
            "default_seconds": policy.sampling_seconds,
            "parked_seconds": policy.parked_sampling_seconds,
        },
        upload={
            "default_seconds": policy.upload_seconds,
            "parked_seconds": policy.parked_upload_seconds,
        },
        vehicle_profile=vehicle.vehicle_profile,
        vehicle_profile_definition=profile_definition(
            db, vehicle.owner_id, vehicle.vehicle_profile
        ),
    )


def create_enrollment(
    db: Session,
    vehicle: Vehicle,
    name: str,
    ttl_minutes: int,
    telemetry_policy: TelemetryPolicy,
) -> tuple[str, EnrollmentToken]:
    raw = new_opaque_token("venroll")
    now = utcnow()
    model = EnrollmentToken(
        token_hash=hash_token(raw),
        vehicle_id=vehicle.id,
        intended_name=name,
        telemetry_policy=telemetry_policy.model_dump(),
        created_at=now,
        expires_at=now + timedelta(minutes=ttl_minutes),
    )
    db.add(model)
    db.flush()
    return raw, model


def enroll(db: Session, request: EnrollRequest) -> EnrollResponse:
    if version_compatibility(request.agent_version) == "incompatible":
        raise EnrollmentError(
            f"agent version {request.agent_version} is incompatible with server {APP_VERSION}; "
            "major versions must match"
        )
    now = utcnow()
    token = db.scalar(
        select(EnrollmentToken)
        .where(EnrollmentToken.token_hash == hash_token(request.token))
        .with_for_update()
    )
    if not token or token.used_at is not None or as_utc(token.expires_at) < now:
        raise EnrollmentError("enrollment token is invalid, expired, or already used")
    vehicle = db.get(Vehicle, token.vehicle_id)
    if not vehicle:
        raise EnrollmentError("vehicle no longer exists")
    credential = new_opaque_token("vdev")
    device = Device(
        vehicle_id=vehicle.id,
        name=token.intended_name,
        credential_hash=hash_token(credential),
        agent_version=request.agent_version,
        hostname=request.hostname,
        hardware=request.hardware,
        telemetry_policy=dict(token.telemetry_policy),
    )
    token.used_at = now
    db.add(device)
    db.flush()
    return EnrollResponse(
        device_id=device.id,
        vehicle_id=vehicle.id,
        credential=credential,
        config=device_config(db, device, vehicle),
    )


def rotate_credential(device: Device) -> str:
    credential = new_opaque_token("vdev")
    device.credential_hash = hash_token(credential)
    device.credential_version += 1
    return credential


def update_telemetry_policy(device: Device, policy: TelemetryPolicy) -> None:
    serialized = policy.model_dump()
    if device.telemetry_policy == serialized:
        return
    device.telemetry_policy = serialized
    device.config_version += 1
