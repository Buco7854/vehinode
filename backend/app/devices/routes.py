from fastapi import APIRouter, HTTPException, status
from sqlalchemy import select

from backend.app.auth.dependencies import CurrentDevice, CurrentUser, CurrentUserWrite, Db
from backend.app.branding import APP_VERSION
from backend.app.common.settings import get_settings
from backend.app.common.time import as_utc, utcnow
from backend.app.devices.models import Device
from backend.app.devices.protocol import (
    registered_agent_implementations,
    registered_agent_installations,
    version_compatibility,
)
from backend.app.devices.schemas import (
    AgentImplementation,
    DeviceConfig,
    DeviceResponse,
    EnrollmentCreate,
    EnrollmentCreated,
    EnrollRequest,
    EnrollResponse,
    RotateCredentialResponse,
    TelemetryPolicy,
)
from backend.app.devices.services import (
    EnrollmentError,
    create_enrollment,
    device_config,
    enroll,
    rotate_credential,
    update_telemetry_policy,
)
from backend.app.vehicles.models import Vehicle
from backend.app.vehicles.services import owned_vehicle

human_router = APIRouter(tags=["devices"])
device_router = APIRouter(prefix="/device", tags=["device API"])


def _owned_device(db: Db, owner_id: str, device_id: str) -> Device:
    device = db.scalar(
        select(Device).join(Vehicle).where(Device.id == device_id, Vehicle.owner_id == owner_id)
    )
    if not device:
        raise HTTPException(status_code=404, detail="device not found")
    return device


@human_router.post(
    "/vehicles/{vehicle_id}/enrollments",
    response_model=EnrollmentCreated,
    status_code=status.HTTP_201_CREATED,
)
def new_enrollment(
    vehicle_id: str, data: EnrollmentCreate, db: Db, auth: CurrentUserWrite
) -> EnrollmentCreated:
    vehicle = owned_vehicle(db, auth.user.id, vehicle_id)
    if not vehicle:
        raise HTTPException(status_code=404, detail="vehicle not found")
    raw, token = create_enrollment(db, vehicle, data.name, data.ttl_minutes, data.telemetry_policy)
    db.commit()
    return EnrollmentCreated(
        token=raw,
        expires_at=token.expires_at,
        server_url=get_settings().public_url.rstrip("/"),
        server_version=APP_VERSION,
        implementations=registered_agent_installations(raw),
    )


@human_router.get("/agent-implementations", response_model=list[AgentImplementation])
def list_agent_implementations(auth: CurrentUser) -> list[AgentImplementation]:
    return [
        AgentImplementation.model_validate(implementation)
        for implementation in registered_agent_implementations()
    ]


@human_router.get("/devices", response_model=list[DeviceResponse])
def list_devices(db: Db, auth: CurrentUser) -> list[DeviceResponse]:
    devices = db.scalars(select(Device).join(Vehicle).where(Vehicle.owner_id == auth.user.id))
    now = utcnow()
    threshold = get_settings().default_online_threshold_seconds
    return [
        DeviceResponse.model_validate(
            {
                **{
                    field: getattr(device, field)
                    for field in DeviceResponse.model_fields
                    if field not in {"online", "version_compatibility"}
                },
                "version_compatibility": version_compatibility(device.agent_version),
                "online": bool(
                    device.revoked_at is None
                    and device.last_seen_at
                    and (now - as_utc(device.last_seen_at)).total_seconds() <= threshold
                ),
            }
        )
        for device in devices
    ]


@human_router.post("/devices/{device_id}/revoke", status_code=status.HTTP_204_NO_CONTENT)
def revoke_device(device_id: str, db: Db, auth: CurrentUserWrite) -> None:
    device = _owned_device(db, auth.user.id, device_id)
    device.revoked_at = utcnow()
    db.commit()


@human_router.post("/devices/{device_id}/rotate", response_model=RotateCredentialResponse)
def rotate_device(device_id: str, db: Db, auth: CurrentUserWrite) -> RotateCredentialResponse:
    device = _owned_device(db, auth.user.id, device_id)
    if device.revoked_at:
        raise HTTPException(status_code=409, detail="revoked device cannot rotate credentials")
    credential = rotate_credential(device)
    db.commit()
    return RotateCredentialResponse(
        credential=credential, credential_version=device.credential_version
    )


@human_router.put("/devices/{device_id}/telemetry-policy", response_model=TelemetryPolicy)
def configure_device(
    device_id: str, data: TelemetryPolicy, db: Db, auth: CurrentUserWrite
) -> TelemetryPolicy:
    device = _owned_device(db, auth.user.id, device_id)
    if device.revoked_at:
        raise HTTPException(status_code=409, detail="revoked device cannot be configured")
    update_telemetry_policy(device, data)
    db.commit()
    return TelemetryPolicy.model_validate(device.telemetry_policy)


@device_router.post("/enroll", response_model=EnrollResponse, status_code=status.HTTP_201_CREATED)
def enroll_device(data: EnrollRequest, db: Db) -> EnrollResponse:
    try:
        response = enroll(db, data)
    except EnrollmentError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    db.commit()
    return response


@device_router.get("/config", response_model=DeviceConfig)
def get_config(device: CurrentDevice, db: Db) -> DeviceConfig:
    vehicle = db.get(Vehicle, device.vehicle_id)
    if not vehicle:
        raise HTTPException(status_code=404, detail="vehicle not found")
    device.last_config_sync_at = utcnow()
    db.commit()
    return device_config(db, device, vehicle)
