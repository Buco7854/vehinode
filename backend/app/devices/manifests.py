"""Discovery of the agent manifests that describe each agent directory.

A manifest declares how one agent is presented in the Devices catalog and whether the hub
hosts its build artifacts. Hosting is a property of the agent, not of the hub: the bundled
Go agent ships prebuilt Linux executables in the image, while an implementation that is
flashed onto a microcontroller may have nothing for the hub to serve at all.
"""

import tomllib
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path
from typing import Literal, cast

SetupKind = Literal["command", "guided"]
MANIFEST_NAME = "agent.toml"
SCHEMA = 1
# Resolves to the repository root in a checkout and to the installed distribution root in
# the image, where the bundled agent's manifest ships as package data next to its installer.
DISTRIBUTION_ROOT = Path(__file__).resolve().parents[3]
# Only a Python package reaches the wheel, so an agent directory that holds no Python has
# its manifest copied here by the image instead. Without it the Devices catalog would list
# an agent in a source checkout and silently omit it from a container deployment.
IMAGE_MANIFEST_DIR = Path("/app/agent-manifests")


class ManifestError(Exception):
    pass


@dataclass(frozen=True)
class ManifestStep:
    """A setup step exactly as written in the manifest, before values are substituted."""

    kind: str
    text: str = ""
    command: str = ""
    value: str = ""
    url: str = ""


@dataclass(frozen=True)
class AgentManifest:
    id: str
    name: str
    hardware: str
    setup_kind: SetupKind
    docs_url: str
    directory: Path
    setup_steps: tuple[ManifestStep, ...]


def _require(table: dict[str, object], key: str, source: Path, kind: type) -> object:
    value = table.get(key)
    if not isinstance(value, kind) or (kind is str and not value):
        raise ManifestError(f"{source}: [{key}] must be a non-empty {kind.__name__}")
    return value


def _parse(source: Path) -> AgentManifest | None:
    try:
        document = tomllib.loads(source.read_text(encoding="utf-8"))
    except (OSError, tomllib.TOMLDecodeError) as reason:
        raise ManifestError(f"{source}: unreadable agent manifest ({reason})") from reason
    # A file that does not announce the manifest schema belongs to something else.
    if document.get("schema") is None:
        return None
    if document.get("schema") != SCHEMA:
        raise ManifestError(f"{source}: unsupported manifest schema {document.get('schema')!r}")

    implementation = document.get("implementation")
    if not isinstance(implementation, dict):
        raise ManifestError(f"{source}: [implementation] is required")
    setup_kind = _require(implementation, "setup_kind", source, str)
    if setup_kind not in ("command", "guided"):
        raise ManifestError(f"{source}: setup_kind must be 'command' or 'guided'")

    setup = document.get("setup")
    raw_steps = setup.get("steps", []) if isinstance(setup, dict) else []
    if not isinstance(raw_steps, list):
        raise ManifestError(f"{source}: [[setup.steps]] must be a list of steps")
    steps = []
    for raw in cast(list[object], raw_steps):
        if not isinstance(raw, dict):
            raise ManifestError(f"{source}: every setup step must be a table")
        unknown = set(raw) - {"kind", "text", "command", "value", "url"}
        if unknown:
            raise ManifestError(f"{source}: unknown setup step key {sorted(unknown)[0]!r}")
        steps.append(ManifestStep(**{key: str(value) for key, value in raw.items()}))

    return AgentManifest(
        id=str(_require(implementation, "id", source, str)),
        name=str(_require(implementation, "name", source, str)),
        hardware=str(_require(implementation, "hardware", source, str)),
        setup_kind=cast(SetupKind, setup_kind),
        docs_url=str(implementation.get("docs_url", "")),
        directory=source.parent,
        setup_steps=tuple(steps),
    )


def discover_manifests(root: Path | None = None) -> tuple[AgentManifest, ...]:
    base = root or DISTRIBUTION_ROOT
    manifests = []
    for source in sorted(base.glob(f"*/{MANIFEST_NAME}")):
        manifest = _parse(source)
        if manifest is not None:
            manifests.append(manifest)
    identifiers = [manifest.id for manifest in manifests]
    if len(set(identifiers)) != len(identifiers):
        raise ManifestError(f"{base}: two agent manifests claim the same implementation id")
    return tuple(manifests)


@lru_cache(maxsize=1)
def agent_manifests() -> tuple[AgentManifest, ...]:
    """Every manifest this deployment can see, wherever its agent directory ended up.

    The same agent may be visible through more than one root — the bundled agent ships in
    the wheel and is copied into the image manifest directory — so the first root to claim
    an implementation id wins instead of being reported as a duplicate.
    """
    found: dict[str, AgentManifest] = {}
    for root in (DISTRIBUTION_ROOT, IMAGE_MANIFEST_DIR):
        if not root.is_dir():
            continue
        for manifest in discover_manifests(root):
            found.setdefault(manifest.id, manifest)
    return tuple(found.values())
