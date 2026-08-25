# Build a custom agent

VehiNode's extension boundary is its device HTTP API, not the bundled Go executable.
An agent may be written in any language and run on any hardware that can make HTTPS
requests. The hub does not import or execute agent code.

There are two ways to use another implementation:

- **Catalog agent:** a reviewed community pull request adds the implementation to the
  Devices page, including the hardware it runs on and the steps that set it up.
- **Custom agent:** choose **Custom agent** and use the displayed server URL, one-time
  token, enrollment endpoint and API reference. No catalog entry or agent identifier is
  required.

In both cases the same token authorizes enrollment and the same device credential realm
protects later requests. Catalog membership grants no additional API permission.

## Test an unreleased implementation

In **Devices**, start an enrollment, choose **Custom agent**, set the telemetry
configuration, and create the disposable enrollment token. The minimum enrollment request
is:

```http
POST /api/v1/device/enroll
Content-Type: application/json

{
  "token": "venroll_...",
  "agent_version": "0.1.0"
}
```

`hostname` and `hardware` are optional support metadata. Enrollment returns the permanent
device credential once; store it securely. The complete generated OpenAPI document is at
`/api/openapi.json`, with an interactive view at `/api/docs`.

From a source checkout, the conformance helper exercises enrollment, configuration,
upload and an idempotent retry:

```sh
python3 scripts/check-agent-protocol.py \
  --server https://vehinode.example \
  --token ONE_TIME_TOKEN \
  --agent-version 0.1.0
```

The check consumes the token and creates real telemetry. It deliberately does not persist
the permanent credential; revoke the resulting test tracker afterward. Plain HTTP is
rejected unless `--allow-http` is explicit.

## Version compatibility

Compatibility is derived from the server and agent semantic versions; there is no
separate protocol-version field.

| Difference | Enrollment | Devices page |
| --- | --- | --- |
| Same major and minor; patch differs | Accepted | No warning |
| Same major; minor differs | Accepted | Orange version warning |
| Major differs | Refused | Red incompatible status for an existing tracker |
| Agent version is not SemVer | Accepted | Orange compatibility-unknown warning |

A major-mismatch rejection happens before the one-time token is consumed, so the token
can still be used by a compatible agent. Clients must ignore unknown response fields so
additive changes do not break them.

## Device lifecycle

1. Exchange the short-lived token with `POST /api/v1/device/enroll`.
2. Store the returned device credential securely; the hub never returns it again.
3. Apply the returned configuration and refresh it with authenticated
   `GET /api/v1/device/config`.
4. Queue samples under stable UUIDs and send batches to
   `POST /api/v1/device/telemetry/batch`.
5. Delete a queued sample only after its UUID appears in either `accepted` or
   `duplicates`. Fetch configuration when a response advertises a newer version.

Authenticated requests use a separate credential realm:

```http
Authorization: Device vdev_...
```

A device credential cannot use human account routes. Never put it in a query parameter,
metric, log, source file, profile or browser storage.

Configuration has a monotonically increasing `version`, driving and parked intervals
mirrored into its sampling and upload sections, and an optional vehicle profile
definition. Validate a candidate fully before replacing the last-known-good configuration. A custom implementation may emit
canonical metrics from its own integration; if it interprets a hub profile, it must not
invent unknown CAN formulas.

Each telemetry batch contains one boot UUID and 1–500 samples. Sample UUIDs are the
idempotency keys. A timeout does not prove failure because the server may have committed
before the response was lost: retry the same UUIDs and treat `duplicates` as success.

## Add an agent to the catalog

Adding an agent takes one file. Create a top-level directory for it holding an `agent.toml`:

```toml
schema = 1

[implementation]
id = "community.esp32"
name = "Community ESP32 firmware"
hardware = "ESP32 with a GPS module"
setup_kind = "guided"          # "command" when one shell command is the whole setup
docs_url = "https://example/docs"

[[setup.steps]]
kind = "link"
text = "Flash the firmware"
url = "https://example/flash"

[[setup.steps]]
kind = "value"
text = "Server URL"
value = "{server}"

[[setup.steps]]
kind = "value"
text = "Enrollment token"
value = "{token}"
```

That is the whole change. Any top-level directory holding an `agent.toml` is an agent
directory: the Devices page picks it up from `GET /api/v1/agent-implementations`, the image
collects the manifest automatically, and no server or frontend code is involved.

`{server}`, `{token}` and `{version}` are substituted when an enrollment is created; inside
a `command` step every substituted value is shell-quoted.

### Setup steps

| Step `kind` | Payload | Rendered as |
| --- | --- | --- |
| `command` | `command` | A copyable shell command |
| `value` | `value` | A copyable value to type into the agent |
| `link` | `url` | A link opened in a new tab |
| `manual` | none | Instruction text only |

`setup_kind` is `command` for an agent that only needs one shell command and `guided` for
anything else; the Devices page shows it with `hardware` before an owner spends a token.
A step carrying the payload its kind cannot render is rejected.

Step `text` is displayed verbatim in every locale, so leave it empty when the hub's own
translated wording for that kind is enough and otherwise keep it to implementation-specific
facts. Put anything longer, including wiring and firmware detail, behind `docs_url`.

An implementation whose steps cannot be written down statically — the bundled agent adds
`--allow-insecure-http` only for a plain-HTTP hub — registers a builder in `SETUP_BUILDERS`
in `backend/app/devices/protocol.py` instead of declaring `[[setup.steps]]`. It needs one or
the other.

### If the hub should ship your binaries

Most agents do not need this: point a setup step at your own releases and you are done. It
is worth doing when trackers must install without reaching the public internet, since the
hub is already on their network.

Add a build stage to the `Dockerfile` that writes your artifacts into `/out`, and copy that
output into the release directory in the runtime stage:

```dockerfile
FROM mcr.microsoft.com/dotnet/sdk:9.0 AS dotnet-agent-build
ARG VEHINODE_VERSION
WORKDIR /src/dotnet-agent
COPY dotnet-agent/ ./
RUN sh build-release.sh "$VEHINODE_VERSION" /out
```

```dockerfile
COPY --from=dotnet-agent-build /out/ /opt/vehinode-agent-releases/${VEHINODE_VERSION}/
```

Keep the release script in your own agent directory; the stage just runs it. Everything in
the release directory is served from
`GET /agent/releases/<version>/<filename>`, so a setup step can install from the hub as soon
as the file is there. `VEHINODE_AGENT_RELEASE_DIR` points at that directory.

To attach the same artifacts to GitHub releases, add a job to
`.github/workflows/agent-release.yml` alongside the Go agent's, building the same way your
Dockerfile stage does.

### Before you open the pull request

1. Pass the custom-agent conformance helper, and add implementation-specific tests for
   credential storage, queue recovery, retries and configuration rollback.
2. Document update, uninstall, artifact-verification and security behavior.
3. Add parser/replay fixtures and update the hardware-validation ledger for any hardware
   integration. Simulator evidence must not be presented as physical validation.

The hub never downloads or executes arbitrary user-supplied plugins. A manifest is in-repo
reviewed content, at the same trust level as the server code around it.
