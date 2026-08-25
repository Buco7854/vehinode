# VehiNode implementation plan

## Phase 1 — usable vertical slice

- [x] Repository structure and durable project context
- [x] PostgreSQL models and Alembic migration
- [x] Local identity, sessions, CSRF, password change/revocation
- [x] First-admin-only registration and idempotent environment bootstrap
- [x] Vehicle creation and ownership enforcement
- [x] One-time device enrollment and separate device authentication
- [x] Idempotent batch telemetry and atomic current state
- [x] Enrollable journey simulator
- [x] Responsive SPA login, vehicle creation and live dashboard
- [x] Tailwind design system, extensible i18n (English/French) and Light/Dark/Auto themes
- [x] Full-viewport live-routebook workspace and dimensionally stable photo-led garage grid
- [x] Browser-language detection and reusable modern select component
- [x] Original node-route mark, instance-focused login and modern open map treatment
- [x] Optional 25 MiB file-backed vehicle photos with an explicit missing-photo placeholder
- [x] Confirmed vehicle deletion with database cascades and photo cleanup
- [x] Authenticated SSE live state with resilient browser reconnection
- [x] Metric-key-driven vehicle presentation without propulsion classification

## Phase 2 — history and dashboards

- [x] Bounded/downsampled history and route API
- [x] History chart, route map and metric selection
- [x] Registry-based dashboard widgets with drag/resize/configuration
- [x] PostgreSQL dashboard persistence
- [x] Premade default Overview plus multiple owner dashboards
- [x] Responsive dashboard grid with stable single-column phone layout
- [x] Normal live dashboard pages with in-place editing and a composed premade Overview
- [x] Shared dashboard vehicle selector with dynamic and explicitly pinned widgets
- [x] Responsive history identity, filter and summary sections with unclipped dropdowns

## Phase 3 — Pi agent

- [x] Installer and systemd unit
- [x] SQLite queue, retry/catch-up and HTTP batch transport
- [x] Versioned last-known-good configuration
- [x] Hub-owned tracker cadence presets and explicit driving/parked interval fields
- [x] Profile/CAN/GPS-aware driving state with grace period and parked heartbeat cadence
- [x] Community-extensible agent catalog plus minimal custom-agent enrollment path
- [x] Hardware-aware catalog entries with per-agent setup steps beyond a shell command
- [x] Single-file agent manifests and a release directory served as the image builds it
- [x] SemVer compatibility policy with refused major and warned minor differences
- [x] CLI diagnostics and simulated providers
- [x] Persistent host-local GPS/OBD selection and confirmation-gated full uninstall
- [x] Standalone CGO-free Go executable for ARMv6, ARMv7, ARM64 and AMD64
- [x] Package-manager-free bootstrap, verified self-update and executable-owned uninstall
- [x] Resumable bootstrap downloads with bounded retry and end-to-end checksum verification

## Phase 4 — physical integrations

- [x] SIM7600 NMEA provider and parser fixtures
- [x] OBDLink SX/STN adapter, standard OBD and reconnection
- [x] Portable CAN capture and offline replay
- [x] Safe declarative profile decoder and experimental C-Zero profile
- [x] Expanded C-Zero READY, charge, motor, body, warning and TPMS mappings with fixtures
- [x] Owner-created profile CRUD, assignment and versioned agent distribution
- [x] Dedicated profiles page with distinct profile/signal modals and no proof-level UX
- [x] Accurate hardware validation ledger
- [ ] Physical SIM7600, OBDLink SX and C-Zero validation (external hardware required)

## Phase 5 — hooks

- [x] Generic event envelope and PostgreSQL durable jobs
- [x] Hook CRUD, permissions and revision recovery
- [x] Child-process runtime, timeout/resource/log limits and crash recovery
- [x] Persistent state, encrypted secrets and central redaction
- [x] Stable context with HTTP, geometry, logging and dry-run helpers
- [x] Execution history, manual/retry APIs and SPA editor with modal creation and compact empty state

## Phase 6 — recipes

- [x] Gate-on-arrival, Traccar and low-SOC hooks

## Phase 7 — production hardening

- [x] Multi-stage non-root Docker image and Compose definition
- [x] Image-only two-file server deployment without a retained source checkout
- [x] Role-aware image entrypoint and declarative Compose service commands
- [x] Health, diagnostics, structured/request-ID logging and payload limits
- [x] Operator-focused VitePress install, usage and operations documentation
- [x] Canonical Compose file import and safe `.env` secret-generation workflow
- [x] CI, GitHub Pages, GHCR and versioned agent release workflows
- [x] Database/media backup and restore procedures plus security threat model
- [x] Full API/agent/hook end-to-end scenario and frontend behavior tests
- [x] Real-browser Playwright E2E with app/worker, mobile and accessibility coverage
- [x] Lockfile-based local CI-equivalent checks available in `scripts/check.sh`
- [x] Real PostgreSQL, Docker image and Compose smoke
- [ ] GitHub Actions, Pages and GHCR publication (requires remote repository execution)
- [ ] Backup/restore exercise against a disposable PostgreSQL deployment
