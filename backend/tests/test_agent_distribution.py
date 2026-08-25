from pathlib import Path
from types import SimpleNamespace

import pytest
from fastapi.testclient import TestClient

from backend.app.devices import protocol


def test_standalone_agent_installer_is_distributed(client: TestClient) -> None:
    installer = client.get("/install-agent")

    assert installer.status_code == 200
    assert "apt-get" not in installer.text
    assert "python3" not in installer.text
    assert 'AGENT_VERSION="0.1.0"' in installer.text
    assert '\nVERSION="0.1.0"' not in installer.text
    assert 'ARTIFACT="vehinode-agent-${AGENT_VERSION}-${TARGET}"' in installer.text
    assert "--continue-at -" in installer.text
    assert 'while [ "$attempt" -le 8 ]' in installer.text
    assert '/usr/local/bin/vehinode-agent "$@"' in installer.text


def test_http_install_command_requires_explicit_insecure_opt_in(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setattr(
        protocol,
        "get_settings",
        lambda: SimpleNamespace(public_url="http://192.168.1.151:8000"),
    )

    steps = protocol.registered_agent_installations("secret token")[0]["setup_steps"]

    assert len(steps) == 1
    command = steps[0]["command"]
    assert "--allow-insecure-http" in command
    assert "'secret token'" in command


def test_release_endpoint_serves_only_known_standalone_targets(
    client: TestClient, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    release = tmp_path / "0.1.0"
    release.mkdir()
    artifact = release / "vehinode-agent-0.1.0-linux-armv6"
    artifact.write_bytes(b"standalone-armv6")
    monkeypatch.setattr(
        "backend.app.api.agent_distribution.get_settings",
        lambda: SimpleNamespace(agent_release_dir=str(tmp_path)),
    )

    response = client.get("/agent/releases/0.1.0/vehinode-agent-0.1.0-linux-armv6")

    assert response.status_code == 200
    assert response.content == b"standalone-armv6"
    assert client.get("/agent/releases/0.1.0/vehinode-0.1.0-py3-none-any.whl").status_code == 404
    assert client.get("/agent/releases/0.1.0/vehinode-agent-0.1.0-linux-mips").status_code == 404
