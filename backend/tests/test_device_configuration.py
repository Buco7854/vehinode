from typing import cast

from fastapi.testclient import TestClient


def _create_vehicle(client: TestClient, csrf: str) -> dict[str, object]:
    response = client.post(
        "/api/v1/vehicles",
        headers={"X-CSRF-Token": csrf},
        json={"name": "Cellular car"},
    )
    assert response.status_code == 201, response.text
    return cast(dict[str, object], response.json())


def _enroll(
    client: TestClient,
    csrf: str,
    vehicle_id: str,
    policy: dict[str, int] | None = None,
) -> dict[str, object]:
    payload: dict[str, object] = {"name": "Low-data Pi"}
    if policy is not None:
        payload["telemetry_policy"] = policy
    enrollment = client.post(
        f"/api/v1/vehicles/{vehicle_id}/enrollments",
        headers={"X-CSRF-Token": csrf},
        json=payload,
    )
    assert enrollment.status_code == 201, enrollment.text
    enrolled = client.post(
        "/api/v1/device/enroll",
        json={
            "token": enrollment.json()["token"],
            "agent_version": "test",
            "hostname": "cellular-pi",
        },
    )
    assert enrolled.status_code == 201, enrolled.text
    return cast(dict[str, object], enrolled.json())


def test_enrollment_policy_reaches_agent_and_can_be_reconfigured(
    registered: tuple[TestClient, str],
) -> None:
    client, csrf = registered
    vehicle = _create_vehicle(client, csrf)
    enrolled = _enroll(
        client,
        csrf,
        str(vehicle["id"]),
        {
            "sampling_seconds": 120,
            "upload_seconds": 120,
            "parked_sampling_seconds": 900,
            "parked_upload_seconds": 900,
        },
    )
    assert enrolled["config"]["sampling"] == {  # type: ignore[index]
        "default_seconds": 120,
        "parked_seconds": 900,
    }
    assert enrolled["config"]["upload"] == {  # type: ignore[index]
        "default_seconds": 120,
        "parked_seconds": 900,
    }

    listed = client.get("/api/v1/devices").json()
    device = listed[0]
    assert device["telemetry_policy"] == {
        "sampling_seconds": 120,
        "upload_seconds": 120,
        "parked_sampling_seconds": 900,
        "parked_upload_seconds": 900,
    }
    assert device["config_version"] == 1

    updated = client.put(
        f"/api/v1/devices/{device['id']}/telemetry-policy",
        headers={"X-CSRF-Token": csrf},
        json={
            "sampling_seconds": 5,
            "upload_seconds": 5,
            "parked_sampling_seconds": 900,
            "parked_upload_seconds": 900,
        },
    )
    assert updated.status_code == 200, updated.text
    assert updated.json() == {
        "sampling_seconds": 5,
        "upload_seconds": 5,
        "parked_sampling_seconds": 900,
        "parked_upload_seconds": 900,
    }

    device_headers = {"Authorization": f"Device {enrolled['credential']}"}
    remote = client.get("/api/v1/device/config", headers=device_headers)
    assert remote.status_code == 200
    assert remote.json()["version"] == 2
    assert remote.json()["sampling"] == {"default_seconds": 5, "parked_seconds": 900}
    assert remote.json()["upload"] == {"default_seconds": 5, "parked_seconds": 900}

    unchanged = client.put(
        f"/api/v1/devices/{device['id']}/telemetry-policy",
        headers={"X-CSRF-Token": csrf},
        json={
            "sampling_seconds": 5,
            "upload_seconds": 5,
            "parked_sampling_seconds": 900,
            "parked_upload_seconds": 900,
        },
    )
    assert unchanged.status_code == 200
    assert client.get("/api/v1/devices").json()[0]["config_version"] == 2


def test_new_enrollments_default_to_data_saver_and_require_matching_intervals(
    registered: tuple[TestClient, str],
) -> None:
    client, csrf = registered
    vehicle = _create_vehicle(client, csrf)
    enrolled = _enroll(client, csrf, str(vehicle["id"]))
    assert enrolled["config"]["sampling"] == {  # type: ignore[index]
        "default_seconds": 120,
        "parked_seconds": 900,
    }
    assert enrolled["config"]["upload"] == {  # type: ignore[index]
        "default_seconds": 120,
        "parked_seconds": 900,
    }

    for policy in (
        {"sampling_seconds": 60, "upload_seconds": 30},
        {
            "sampling_seconds": 30,
            "upload_seconds": 30,
            "parked_sampling_seconds": 900,
            "parked_upload_seconds": 800,
        },
    ):
        response = client.post(
            f"/api/v1/vehicles/{vehicle['id']}/enrollments",
            headers={"X-CSRF-Token": csrf},
            json={"name": "Invalid tracker", "telemetry_policy": policy},
        )
        assert response.status_code == 422
