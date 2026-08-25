# ADR 0008: Agent catalog and API compatibility

## Status

Accepted

## Context

VehiNode ships a CGO-free Go tracker for Raspberry Pi-class Linux hardware, but operators
may build collectors in another language or for another device. Requiring a server-side
record for every implementation would block prototypes and couple telemetry to one
executable. Loading third-party tracker code into the API or worker would expand the
trusted runtime and weaken the device credential boundary.

The application still needs a curated way to present implementations that the community
has reviewed, while allowing an author to test code before its catalog pull request is
accepted.

## Decision

The `/api/v1/device` HTTP API is the extension boundary. Its enrollment request has only
two required fields: a valid one-time `token` and the agent's `agent_version`. `hostname`
and `hardware` are optional metadata. There is no agent ID and no separate protocol
version.

The Devices page loads a server-owned catalog of reviewed choices before enrollment so the
implementation, vehicle, tracker name and telemetry configuration are selected together.
Catalog entries are not equivalent to one another: an agent runs on specific hardware and
is set up in its own way, so each agent directory declares — in an `agent.toml` manifest —
the hardware it supports and ordered setup steps rather than a shell command the hub
assumes every tracker can run. Shipping an agent's binaries is a separate concern the
manifest says nothing about: a build stage writes them into the release directory, and
whatever is in that directory is served. A
step is a copyable command, a copyable value, a link or plain instruction text; the
bundled Go agent returns exactly one command step, while firmware for a microcontroller
can return a flashing link followed by the values to enter on the device. A **Custom
agent** option instead displays the server URL, one-time token, minimum enrollment fields
and OpenAPI reference. Catalog membership is presentation metadata, not authorization;
possession of the valid token is what permits enrollment.

Compatibility is derived from SemVer:

- patch differences within the same major and minor are compatible;
- minor differences within the same major are accepted with an orange warning;
- major differences are incompatible and enrollment is refused before consuming the
  token; and
- a non-SemVer agent version is accepted with an orange unknown-compatibility warning.

Existing enrolled trackers are evaluated against the running server version when listed,
so an old major version appears red after a major server upgrade. Clients must ignore
unknown JSON response fields to tolerate additive changes.

## Consequences

- An agent can be implemented and tested independently in any language.
- The hub never imports or executes custom agent code.
- A community pull request can add a reviewed catalog entry, including one for hardware
  that no shell command can install, by adding a single manifest file: no authorization
  realm, frontend integration, server code or image change. Server code is needed only when
  setup steps cannot be written down statically, and an image change only when the hub is
  asked to host that agent's binaries.
- An agent that needs trackers to install without internet access adds a build stage whose
  output lands in the release directory; anything else links to its own release hosting and
  adds nothing to the image.
- The release directory holds exactly what the image build put there, so it is served as-is
  rather than against a declared list that would have to be kept in step with the build.
- Setup step text supplied by a catalog entry is not translated; entries leave it empty to
  use the hub's own translated wording and keep longer detail behind their documentation
  link.
- The roster retains only the self-reported version needed for compatibility diagnostics.
- The conformance flow exercises enrollment, configuration, upload and idempotent retry
  before an implementation is added to the catalog.
- Major compatibility is intentionally strict; minor differences remain testable and
  visible rather than being blocked.
