# Current project state

Updated: 2026-08-25

## Works

- The modular monolith implements local session authentication, vehicle ownership,
  one-time device enrollment, separate device credentials, idempotent telemetry,
  current state, bounded history, dashboards, diagnostics and explicit migrations.
  Local registration can create only the first administrator. That account can instead
  be bootstrapped idempotently from environment variables; later registration is always
  rejected and the identity boundary remains ready for a future OIDC provider.
- The Tailwind Vue SPA uses a full-viewport live-routebook workspace with a clear,
  full-height sidebar and one dashboard destination. A versioned premade Overview, multiple
  owner dashboards, responsive single-column mobile widgets, route/history charts,
  tracker administration, hooks and settings are functional. The searchable garage
  uses dimensionally stable photo cards and leaves optional vehicle facts blank. Self-hosted
  IBM Plex typography, an original node-route mark, a modern neutralized OpenStreetMap treatment,
  optional owner-scoped vehicle photos with a plain missing-image placeholder, a
  cobalt recorded-route accent,
  extensible English/French catalogs, browser-language detection, app-owned accessible
  dropdowns, modal creation flows and persistent Light, Dark and Auto themes work across
  the application.
  Login copy is operational and instance-focused rather than promotional.
- Vehicle deletion uses an explicit confirmation modal, removes the vehicle's uploaded
  photo, cascades dependent tracker credentials and telemetry in PostgreSQL, and leaves
  reusable dashboard widgets present but no longer pinned to the deleted vehicle.
- Vehicles have no propulsion classification in the UI, API or current schema. Garage,
  dashboards, history and widgets choose their presentation only from metric keys that
  are actually present; battery and fuel readings may coexist, and missing readings
  remain neutral rather than 0%.
- Dashboards render as normal live pages; one overflow menu opens edit/create actions and
  edit mode adds controls directly to the same canvas. The versioned Overview is composed
  from ordinary selector, map, media, energy, telemetry, chart and tracker widgets and is
  added without removing older dashboards. Unpinned widgets react to the shared vehicle
  selector while explicitly pinned widgets remain fixed. The selector is a bounded,
  searchable dropdown for large fleets. Data widgets share a deliberate no-data state,
  avoid mounting empty maps/charts, and omit unavailable telemetry rows.
  History keeps identity, filters and summary data in responsive sections. Vehicle and
  tracker creation use focused modals. Profiles have their own routed page, full-width
  profile rows, aligned vertical details, and distinct profile/signal modals with no
  artificial user-facing proof level. Hook creation also uses a focused modal and the
  page no longer renders an unused blank editor before the first hook.
- Durable PostgreSQL jobs invoke trusted hooks in limited child processes outside API
  requests. Hooks have revisions, state, encrypted write-only secrets, redacted logs,
  HTTP/geometry helpers, manual dry-run and execution history.
- The deployed vehicle agent is a standalone CGO-free Go executable. Versioned Linux
  builds cover ARMv6, ARMv7, ARM64 and AMD64; the bootstrap downloads the matching
  checksum-verified artifact without running `apt`, Python or a compiler on the tracker.
  It implements enrollment, a compiled-in offline SQLite outbox, remote last-known-good
  configuration, SIM7600 NMEA parsing, OBDLink/OBD support, safe profiles, CAN
  capture/replay, diagnostics, installation and systemd integration.
  Host-local hardware selection persists GPS and OBD as `auto`, `off`, or a verified
  stable `/dev/serial/by-id` path; diagnostics expose candidates without claiming that a
  name alone proves the protocol. Installation grants serial access and the executable
  retries and resumes interrupted downloads before checksum verification. The executable
  provides checksum-verified self-updates plus confirmation-gated complete removal of the
  service, executable, credentials and queued telemetry.
  Owners can create declarative profiles in the SPA; definitions are owner-scoped,
  server-validated, versioned with assigned devices, and validated again by the agent
  before replacing last-known-good configuration. Built-in profiles remain read-only.
  Tracker enrollment persists a hub-owned driving/parked policy with data-saver,
  balanced, light-live and live presets plus directly editable driving and parked
  interval fields. Sampling and upload always use the same interval in each state; the
  built-in presets use a fifteen-minute parked heartbeat. The agent
  observes motion evidence every second without durably queuing every observation:
  freshly decoded canonical READY/ignition and CAN speed take precedence, GPS is the
  fallback, and zero speed receives a five-minute grace period. Owners can reconfigure
  enrolled trackers; changes increment the device configuration version. The Go agent
  learns that version from telemetry acknowledgements and fetches full configuration only
  on change, with a six-hour fallback sync for metered connections.
  Reviewed community agents can be added through a server-owned setup catalog that the SPA
  renders dynamically. Every top-level directory holding an `agent.toml` manifest is an
  agent directory, and that one file is the whole change: catalog identity, the hardware it
  runs on, and ordered setup steps—a copyable command, a copyable value, a link or
  instruction text. An implementation that is flashed and provisioned on the device is
  presented as correctly as the bundled Go agent's single command, and needs no server,
  frontend or image change; server code is needed only when steps require conditional logic.
  A generic image stage collects every manifest, so an agent written in any language is
  listed even though only a Python package reaches the wheel. The Devices page shows the
  hardware and whether setup is one command or guided steps before a token is spent.
  Shipping binaries from the hub is separate and needs no manifest metadata: a build stage
  in the Dockerfile writes artifacts into `/opt/vehinode-agent-releases/<version>/`, and
  everything in that directory is served from `GET /agent/releases/<version>/<filename>`,
  bounded so neither path segment can leave the directory.
  An unreleased custom agent instead receives the server URL,
  one-time token, minimum `token` + `agent_version` enrollment fields and OpenAPI reference;
  there is no wire-level agent ID or protocol-version field. Compatibility is derived from
  SemVer: patch differences are silent, minor differences are accepted with an orange
  warning, major differences are red and refused before consuming the token, and invalid
  SemVer is accepted with an orange unknown-compatibility warning.
  The C-Zero profile now covers 36 passive signals, including the supplied battery/cell,
  charge, READY, motor, range, body, warning and per-wheel pressure/temperature mappings;
  each formula has a synthetic fixture. Active diagnostic requests remain excluded.
- Production artifacts include a non-root multi-stage image, three-service Compose,
  CI/Pages/GHCR/release workflows, operator-focused VitePress docs and backup/restore
  scripts. A deployed server needs only the image-based Compose file and its private
  `.env`; it does not retain the source tree or require helper scripts. The Docker guide
  imports the canonical Compose file and documents direct backup/restore commands. A
  role-aware image entrypoint owns migrations and app/worker startup; Compose contains
  no project `name` or embedded shell startup pipeline. Vehicle photo bytes are capped
  at 25 MiB and stored in a dedicated filesystem volume; PostgreSQL stores metadata
  only, and the backup/restore procedure covers both stores.

## Verification

- Ruff and Ruff format pass across backend/agent; mypy passes for 109 source files in Linux.
- Backend/agent tests runnable without PostgreSQL pass on Linux, including vehicle photo
  validation/storage/ownership coverage and the complete
  simulator-to-hook E2E scenario, custom-profile distribution and ownership, minimum
  custom-agent enrollment, SemVer compatibility and idempotent custom-agent upload retry.
- Frontend: ESLint and strict type check passing; 6 files / 28 behavior tests passing;
  production build passing.
- Playwright: 2 Chromium scenarios passing locally against a fresh migrated database,
  real API and worker. CI runs the same suite on PostgreSQL. They cover the primary
  product journey, idempotency, auth-realm isolation, live SSE state changes,
  environment-based admin bootstrap, rejection of later registration, file-backed photo
  upload with invariant card height, multiple/default dashboard persistence, mobile
  widget reflow, the routed profile/modal flow, browser-language detection, EN/FR,
  themes, metric-key-driven rendering
  and automated axe checks. The
  expanded stale-vehicle check found and fixed a light-theme status contrast defect. The
  composed dashboard selector also passes axe after correcting its grouped-button semantics.
  Tracker configuration passes keyboard preset selection, narrow-dialog reflow and axe,
  including the four presets and directly editable driving/parked interval fields. The
  custom-agent enrollment facts pass keyboard selection, desktop/mobile reflow and axe;
  rendered checks also corrected low-contrast labels and removed a duplicate banner
  landmark from the shared modal shell.
- The Go agent passes format, vet, unit tests and CGO-free cross-builds for all four
  release targets. Every packaged artifact has a matching verified SHA-256 checksum and
  the Linux AMD64 executable runs from the production image.
- VitePress build (including the repository Compose import), secret-file generation
  smoke test and Python server wheel build pass. Alembic upgrade/check/downgrade passes with
  the local SQLite migration smoke database.
- The committed lockfiles install from a fresh checkout and `scripts/check.sh` resolves
  the checkout directly; no prior editable installation is required for validation.
- The production image builds on Docker Desktop. Compose is running against PostgreSQL
  with migration `91c5e8a3f204` from the prior image; local migration `d4e5f6a7b8c9`
  passes upgrade/check/downgrade and awaits the next image deployment. App/PostgreSQL are
  healthy and the worker uses
  its role-appropriate no-HTTP health policy. The packaged image serves the bootstrap and
  all four standalone agent targets; lifecycle operations live in the executable. The
  browser suite passes against a disposable Linux image/SQLite app and worker. GitHub
  publication still requires the remote.

## Broken or failing

- No known runnable test or build failure.
- External validation remains: CI on GitHub, GHCR/Pages publication, backup/restore
  against a disposable deployment, and physical hardware/car.

## Hardware validation

- No hardware behavior is claimed physically verified. SIM7600, OBDLink and C-Zero
  paths are implementation/fixture-tested only; C-Zero signals remain experimental or
  unknown according to their source evidence.
  See `docs/agent/hardware-validation.md`.

## Exact next action

Exercise backup/restore against a disposable deployment. Run the GitHub workflows from
the remote repository. Validate hardware using the ledger without promoting experimental
signal status prematurely.
