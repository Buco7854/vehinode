import re
import shlex
from collections.abc import Callable
from dataclasses import dataclass
from typing import Literal, TypedDict, cast

from backend.app.branding import APP_VERSION
from backend.app.common.settings import get_settings
from backend.app.devices.manifests import AgentManifest, ManifestError, SetupKind, agent_manifests

BUNDLED_AGENT_IMPLEMENTATION = "vehinode.go"
SetupStepKind = Literal["command", "value", "link", "manual"]
VersionCompatibility = Literal["compatible", "warning", "incompatible", "unknown"]
SEMVER = re.compile(r"^(\d+)\.(\d+)\.(\d+)(?:[-+][0-9A-Za-z.-]+)?$")
STEP_PAYLOAD: dict[str, str] = {"command": "command", "value": "value", "link": "url"}


class DescribedAgent(TypedDict):
    id: str
    name: str
    hardware: str
    setup_kind: SetupKind
    docs_url: str


class RenderedSetupStep(TypedDict):
    kind: SetupStepKind
    text: str
    command: str
    value: str
    url: str


class AgentInstallation(DescribedAgent):
    setup_steps: list[RenderedSetupStep]


@dataclass(frozen=True)
class SetupStep:
    """One reviewed instruction. `kind` selects the affordance the SPA renders.

    `text` is optional for a step whose affordance already carries its meaning; the SPA
    then shows its own translated wording for that kind. Text supplied here is displayed
    verbatim in every locale, so keep it to implementation-specific facts.
    """

    kind: SetupStepKind
    text: str = ""
    command: str = ""
    value: str = ""
    url: str = ""

    def __post_init__(self) -> None:
        carried = {field for field in ("command", "value", "url") if getattr(self, field)}
        expected = {STEP_PAYLOAD[self.kind]} if self.kind in STEP_PAYLOAD else set()
        if carried != expected:
            raise ValueError(f"a {self.kind} step must carry exactly {expected or 'no payload'}")
        if self.kind == "manual" and not self.text.strip():
            raise ValueError("a manual step must describe what to do")


def _bundled_go_setup(manifest: AgentManifest, token: str) -> tuple[SetupStep, ...]:
    """The bundled agent picks its own installer flags, so its steps stay reviewed code."""
    base = get_settings().public_url.rstrip("/")
    installer_url = shlex.quote(f"{base}/install-agent")
    insecure = " --allow-insecure-http" if base.startswith("http://") else ""
    return (
        SetupStep(
            kind="command",
            command=(
                f"curl -fsSL {installer_url} | sudo sh -s -- "
                f"--server {shlex.quote(base)} --token {shlex.quote(token)} "
                f"--version {APP_VERSION}{insecure}"
            ),
        ),
    )


# An implementation only needs an entry here when its setup cannot be written down as
# static steps in its manifest. Everything else is rendered from the manifest itself.
SETUP_BUILDERS: dict[str, Callable[[AgentManifest, str], tuple[SetupStep, ...]]] = {
    BUNDLED_AGENT_IMPLEMENTATION: _bundled_go_setup,
}


def _manifest_setup(manifest: AgentManifest, token: str) -> tuple[SetupStep, ...]:
    base = get_settings().public_url.rstrip("/")
    plain = {"server": base, "token": token, "version": APP_VERSION}
    # A command step is pasted into a shell, so every substituted value is quoted there.
    quoted = {key: shlex.quote(value) for key, value in plain.items()}
    steps = []
    for step in manifest.setup_steps:
        values = quoted if step.kind == "command" else plain
        steps.append(
            SetupStep(
                kind=cast(SetupStepKind, step.kind),
                text=step.text.format(**plain),
                command=step.command.format(**values),
                value=step.value.format(**plain),
                url=step.url.format(**plain),
            )
        )
    return tuple(steps)


def setup_steps(manifest: AgentManifest, token: str) -> tuple[SetupStep, ...]:
    builder = SETUP_BUILDERS.get(manifest.id)
    if builder:
        return builder(manifest, token)
    if not manifest.setup_steps:
        raise ManifestError(f"{manifest.id}: declare [[setup.steps]] or register a setup builder")
    return _manifest_setup(manifest, token)


def _described(manifest: AgentManifest) -> DescribedAgent:
    return {
        "id": manifest.id,
        "name": manifest.name,
        "hardware": manifest.hardware,
        "setup_kind": manifest.setup_kind,
        "docs_url": manifest.docs_url,
    }


def _rendered(step: SetupStep) -> RenderedSetupStep:
    return {
        "kind": step.kind,
        "text": step.text,
        "command": step.command,
        "value": step.value,
        "url": step.url,
    }


def registered_agent_implementations() -> list[DescribedAgent]:
    return [_described(manifest) for manifest in agent_manifests()]


def registered_agent_installations(token: str) -> list[AgentInstallation]:
    return [
        {
            **_described(manifest),
            "setup_steps": [_rendered(step) for step in setup_steps(manifest, token)],
        }
        for manifest in agent_manifests()
    ]


def version_compatibility(agent_version: str | None) -> VersionCompatibility:
    server = SEMVER.fullmatch(APP_VERSION)
    agent = SEMVER.fullmatch(agent_version or "")
    if not server or not agent:
        return "unknown"
    if agent.group(1) != server.group(1):
        return "incompatible"
    if agent.group(2) != server.group(2):
        return "warning"
    return "compatible"
