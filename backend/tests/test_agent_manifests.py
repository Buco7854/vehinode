from pathlib import Path

import pytest

from backend.app.devices.manifests import ManifestError, discover_manifests

MANIFEST = """
schema = 1

[implementation]
id = "example.agent"
name = "Example agent"
hardware = "Linux boards"
setup_kind = "command"

[[setup.steps]]
kind = "command"
text = "Install it"
command = "install --server {server} --token {token}"
"""


def _write(root: Path, name: str, manifest: str) -> Path:
    directory = root / name
    directory.mkdir(parents=True, exist_ok=True)
    (directory / "agent.toml").write_text(manifest)
    return directory


def test_a_manifest_is_all_an_agent_directory_needs(tmp_path: Path) -> None:
    _write(tmp_path, "example-agent", MANIFEST)

    manifest = discover_manifests(tmp_path)[0]

    assert manifest.id == "example.agent"
    assert manifest.setup_kind == "command"
    assert manifest.directory.name == "example-agent"
    assert [step.kind for step in manifest.setup_steps] == ["command"]


def test_a_file_that_is_not_a_vehinode_manifest_is_ignored(tmp_path: Path) -> None:
    _write(tmp_path, "unrelated", "[tool.something]\nvalue = 1\n")

    assert discover_manifests(tmp_path) == ()


@pytest.mark.parametrize(
    ("mutation", "replacement"),
    [
        ('setup_kind = "command"', 'setup_kind = "occasionally"'),
        ('id = "example.agent"', 'id = ""'),
        ('name = "Example agent"', "name = 1"),
        ('kind = "command"', 'kind = "command"\nunexpected = "key"'),
        ("schema = 1", "schema = 99"),
    ],
)
def test_a_manifest_that_cannot_be_honored_is_refused(
    tmp_path: Path, mutation: str, replacement: str
) -> None:
    _write(tmp_path, "example-agent", MANIFEST.replace(mutation, replacement))

    with pytest.raises(ManifestError):
        discover_manifests(tmp_path)


def test_two_agents_cannot_claim_the_same_implementation_id(tmp_path: Path) -> None:
    _write(tmp_path, "first-agent", MANIFEST)
    _write(tmp_path, "second-agent", MANIFEST)

    with pytest.raises(ManifestError):
        discover_manifests(tmp_path)


def test_an_agent_directory_without_python_is_still_visible_in_the_image(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    from backend.app.devices import manifests

    checkout = tmp_path / "checkout"
    image_manifests = tmp_path / "app" / "agent-manifests"
    # The bundled agent reaches the image twice: through the wheel and through the copy.
    _write(checkout, "agent", MANIFEST)
    _write(image_manifests, "agent", MANIFEST)
    _write(image_manifests, "dotnet-agent", MANIFEST.replace("example.agent", "community.dotnet"))
    monkeypatch.setattr(manifests, "DISTRIBUTION_ROOT", checkout)
    monkeypatch.setattr(manifests, "IMAGE_MANIFEST_DIR", image_manifests)
    manifests.agent_manifests.cache_clear()

    try:
        assert [manifest.id for manifest in manifests.agent_manifests()] == [
            "example.agent",
            "community.dotnet",
        ]
    finally:
        manifests.agent_manifests.cache_clear()
