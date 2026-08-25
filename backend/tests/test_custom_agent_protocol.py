from datetime import UTC, datetime
from pathlib import Path
from types import SimpleNamespace
from typing import cast
from uuid import uuid4

import pytest
from fastapi import HTTPException
from fastapi.testclient import TestClient

from backend.app.api.agent_distribution import agent_release
from backend.app.devices import protocol
from backend.app.devices.manifests import ManifestError, discover_manifests
from backend.app.devices.protocol import SetupStep, version_compatibility


def _vehicle(client: TestClient, csrf: str) -> dict[str, object]:
    response = client.post(
        "/api/v1/vehicles",
        headers={"X-CSRF-Token": csrf},
        json={"name": "Agent test vehicle"},
    )
    assert response.status_code == 201, response.text
    return response.json()  # type: ignore[no-any-return]


def _token(client: TestClient, csrf: str, vehicle_id: object) -> dict[str, object]:
    response = client.post(
        f"/api/v1/vehicles/{vehicle_id}/enrollments",
        headers={"X-CSRF-Token": csrf},
        json={"name": "Custom tracker"},
    )
    assert response.status_code == 201, response.text
    return response.json()  # type: ignore[no-any-return]


def _setup_steps(invitation: dict[str, object]) -> list[dict[str, str]]:
    implementations = cast(list[dict[str, object]], invitation["implementations"])
    return cast(list[dict[str, str]], implementations[0]["setup_steps"])


def test_custom_agent_can_enroll_with_minimum_payload_and_retry_telemetry(
    registered: tuple[TestClient, str],
) -> None:
    client, csrf = registered
    implementations = client.get("/api/v1/agent-implementations")
    assert implementations.status_code == 200
    catalog = implementations.json()
    assert [entry["id"] for entry in catalog] == ["vehinode.go"]
    assert catalog[0]["setup_kind"] == "command"
    assert "Raspberry Pi" in catalog[0]["hardware"]
    vehicle = _vehicle(client, csrf)
    invitation = _token(client, csrf, vehicle["id"])
    assert invitation["server_url"]
    assert invitation["server_version"] == "0.1.0"
    assert invitation["implementations"] == [
        {
            **catalog[0],
            "setup_steps": [
                {
                    "kind": "command",
                    "text": "",
                    "command": _setup_steps(invitation)[0]["command"],
                    "value": "",
                    "url": "",
                }
            ],
        }
    ]
    assert "install_command" not in invitation

    enrollment = client.post(
        "/api/v1/device/enroll",
        json={"token": invitation["token"], "agent_version": "0.2.0"},
    )
    assert enrollment.status_code == 201, enrollment.text
    enrolled = enrollment.json()
    headers = {"Authorization": f"Device {enrolled['credential']}"}
    assert client.get("/api/v1/device/config", headers=headers).status_code == 200

    sample_id = str(uuid4())
    batch = {
        "boot_id": str(uuid4()),
        "samples": [
            {
                "id": sample_id,
                "sequence": 1,
                "recorded_at": datetime.now(UTC).isoformat(),
                "metrics": {"diagnostic.conformance": True},
                "device": {"agent.conformance": True},
            }
        ],
    }
    accepted = client.post("/api/v1/device/telemetry/batch", headers=headers, json=batch)
    assert accepted.status_code == 200
    assert accepted.json()["accepted"] == [sample_id]
    duplicate = client.post("/api/v1/device/telemetry/batch", headers=headers, json=batch)
    assert duplicate.status_code == 200
    assert duplicate.json()["duplicates"] == [sample_id]

    devices = client.get("/api/v1/devices").json()
    assert devices[0]["agent_version"] == "0.2.0"
    assert devices[0]["version_compatibility"] == "warning"
    assert devices[0]["hostname"] == ""
    assert devices[0]["hardware"] == {}


def test_major_mismatch_does_not_consume_enrollment_token(
    registered: tuple[TestClient, str],
) -> None:
    client, csrf = registered
    vehicle = _vehicle(client, csrf)
    invitation = _token(client, csrf, vehicle["id"])
    request = {"token": invitation["token"], "agent_version": "2.0.0"}

    rejected = client.post("/api/v1/device/enroll", json=request)
    assert rejected.status_code == 400
    assert "major versions must match" in rejected.json()["error"]["message"]

    request["agent_version"] = "0.1.9"
    accepted = client.post("/api/v1/device/enroll", json=request)
    assert accepted.status_code == 201, accepted.text


@pytest.mark.parametrize(
    ("agent_version", "expected"),
    [
        ("0.1.9", "compatible"),
        ("0.2.0", "warning"),
        ("1.1.0", "incompatible"),
        ("nightly", "unknown"),
    ],
)
def test_version_compatibility(agent_version: str, expected: str) -> None:
    assert version_compatibility(agent_version) == expected


FIRMWARE_MANIFEST = """
schema = 1

[implementation]
id = "community.esp32"
name = "Community ESP32 firmware"
hardware = "ESP32 with a GPS module"
setup_kind = "guided"
docs_url = "https://example.invalid/docs"

[[setup.steps]]
kind = "link"
text = "Flash the firmware"
url = "https://example.invalid/flash"

[[setup.steps]]
kind = "manual"
text = "Join the VehiNode-Setup access point"

[[setup.steps]]
kind = "value"
text = "Server URL"
value = "{server}"

[[setup.steps]]
kind = "value"
text = "Enrollment token"
value = "{token}"
"""


def _catalog_from(path: Path, manifest: str, monkeypatch: pytest.MonkeyPatch) -> None:
    directory = path / "esp32-agent"
    directory.mkdir()
    (directory / "agent.toml").write_text(manifest)
    monkeypatch.setattr(protocol, "agent_manifests", lambda: discover_manifests(path))


def test_a_manifest_alone_publishes_an_agent_that_no_command_can_install(
    registered: tuple[TestClient, str], tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    client, csrf = registered
    _catalog_from(tmp_path, FIRMWARE_MANIFEST, monkeypatch)

    catalog = client.get("/api/v1/agent-implementations").json()
    assert catalog == [
        {
            "id": "community.esp32",
            "name": "Community ESP32 firmware",
            "hardware": "ESP32 with a GPS module",
            "setup_kind": "guided",
            "docs_url": "https://example.invalid/docs",
        }
    ]

    vehicle = _vehicle(client, csrf)
    invitation = _token(client, csrf, vehicle["id"])
    steps = _setup_steps(invitation)
    assert [step["kind"] for step in steps] == ["link", "manual", "value", "value"]
    assert not any(step["command"] for step in steps)
    assert steps[2]["value"].startswith("http")
    assert steps[3]["value"] == invitation["token"]


def test_the_release_directory_is_served_but_cannot_be_walked_out_of(
    client: TestClient, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    releases = tmp_path / "releases" / "0.1.0"
    releases.mkdir(parents=True)
    (releases / "vehinode-agent-0.1.0-linux-amd64").write_bytes(b"bundled")
    (releases / "community-firmware-0.1.0.bin").write_bytes(b"firmware")
    (tmp_path / "releases" / "secret").write_bytes(b"not a release")
    monkeypatch.setattr(
        "backend.app.api.agent_distribution.get_settings",
        lambda: SimpleNamespace(agent_release_dir=str(tmp_path / "releases")),
    )

    # Whatever the image build put in the directory is installable, whichever agent it is.
    assert client.get("/agent/releases/0.1.0/vehinode-agent-0.1.0-linux-amd64").status_code == 200
    assert client.get("/agent/releases/0.1.0/community-firmware-0.1.0.bin").status_code == 200
    assert client.get("/agent/releases/0.1.0/missing").status_code == 404

    # A filename is one plain segment, so neither parameter can reach outside the directory.
    for escape in ("../secret", "..", ".hidden", "sub/file", ""):
        with pytest.raises(HTTPException):
            agent_release("0.1.0", escape)
    with pytest.raises(HTTPException):
        agent_release("../..", "vehinode-agent-0.1.0-linux-amd64")


def test_an_agent_without_steps_or_a_builder_is_refused(
    registered: tuple[TestClient, str], tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    client, csrf = registered
    _catalog_from(tmp_path, FIRMWARE_MANIFEST.split("[[setup.steps]]")[0], monkeypatch)
    vehicle = _vehicle(client, csrf)

    with pytest.raises(ManifestError):
        client.post(
            f"/api/v1/vehicles/{vehicle['id']}/enrollments",
            headers={"X-CSRF-Token": csrf},
            json={"name": "Unbuildable"},
        )


@pytest.mark.parametrize(
    "step",
    [
        {"kind": "command", "value": "not a command"},
        {"kind": "link", "text": "Flash the firmware"},
        {"kind": "value", "text": "Server URL", "value": "https://hub.example", "url": "https://x"},
        {"kind": "manual", "text": "   "},
    ],
)
def test_setup_step_rejects_a_payload_its_kind_cannot_render(step: dict[str, str]) -> None:
    with pytest.raises(ValueError):
        SetupStep(**step)  # type: ignore[arg-type]
