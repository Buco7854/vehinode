# Telemetry ingestion

`POST /api/v1/device/telemetry/batch` accepts up to 500 samples. Each carries a stable
UUID, boot-local sequence, UTC timestamp, optional position, canonical metric map and
device-health map. PostgreSQL uniquely constrains sample UUIDs; a retry after a lost
response reports the row as duplicate without changing history or rerunning hooks.
The response also carries the device configuration version, allowing the agent to fetch
configuration only when it changes instead of repeatedly downloading the full profile.
This endpoint is not tied to the bundled Go executable. Custom implementations use the
same authentication, schema and idempotency rules; see
[Build a custom agent](./custom-agents.md).

The Go agent separates one-second movement observation from durable reporting. Canonical
`vehicle.ready`/`vehicle.ignition` evidence takes priority, followed by fresh CAN speed and
GPS motion. A zero-speed observation only starts a five-minute inactivity timer. The
selected configuration supplies separate driving and parked intervals. Sampling and
upload use the same interval in each state, so normal operation sends each durable point
immediately while retaining the SQLite retry queue for failures.

Position and common query dimensions are relational columns. Variable canonical
metrics remain JSONB. The newest recorded sample updates `vehicle_state`, making live
dashboards cheap. Older delayed samples are stored in history but cannot rewind current
state.

History requests have a bounded range and result size. The service reduces dense data
server-side before returning route/chart points. PostgreSQL remains the only time-series
store at the intended 1–100 vehicle scale.

## Live browser state

Authenticated browsers subscribe to `GET /api/v1/events/stream`, a same-origin
Server-Sent Events stream. The server sends a versioned `vehicle.states` snapshot when
owned current state changes and a comment heartbeat while idle. The dashboard updates
its state immediately and refreshes route history only for a newly received sample;
there is no fixed browser polling interval.

The stream uses the opaque browser session cookie, revalidates that session while it is
open, and emits `session.expired` before closing a revoked or expired connection. Device
credentials cannot open it. Snapshots come from PostgreSQL rather than process-local
memory, so multiple app processes remain consistent. The EventSource client reconnects
automatically after temporary network or server failures.
