# Devices and enrollment

Choose the vehicle, tracker name, agent implementation and telemetry cadence together in
the enrollment dialog. While driving, **Data saver** records and uploads every two minutes,
**Balanced** every thirty seconds, **Light live** every five seconds, and **Live** every
second. Every preset records and uploads a parked heartbeat every fifteen minutes. Data
saver is the default for small cellular plans. During tracker creation, the selected
preset fills two ordinary fields: the driving interval and the parked interval. Both can
be edited directly. In each state the agent samples and uploads at that same interval;
the durable queue still retains points when an upload fails.

Choose a reviewed agent from the server-owned catalog, or choose **Custom agent** for an
implementation that is not yet listed. Agents are not interchangeable: each catalog entry
states the hardware it runs on and whether setup is a single command or guided steps, so
that choice is visible before a token is spent. Only after all enrollment settings are
submitted does the same dialog reveal that agent's setup steps, or the custom agent's
server URL, one-time token, minimum `token` + `agent_version` enrollment fields and API
reference.

The bundled Go agent for the Raspberry Pi and other Linux boards is one copyable command.
Another agent may instead need firmware flashed and the server URL and token entered on
the device, in which case the dialog lists those steps in order with the values to copy
and a link to that agent's own documentation.

Agent and server versions use a simple compatibility rule: patch differences are accepted
silently, minor differences are accepted with an orange warning, and major differences
are incompatible, shown in red and refused during enrollment. A custom version that is
not valid SemVer is accepted with an orange compatibility-unknown warning.

The agent observes movement evidence every second without automatically making each
observation durable. A freshly decoded profile metric named `vehicle.ready` or
`vehicle.ignition` has priority, followed by fresh CAN vehicle speed and GPS
speed/displacement. Zero speed does not immediately mean parked: without an explicit
off signal, five continuous minutes without movement are required. Any movement returns
the tracker to its driving cadence immediately.

For planning, estimates using 50 driving hours/month, fifteen-minute parked heartbeats,
GPS, the expanded C-Zero metric set and current JSON/HTTP protocol are roughly 9 MB/month
for Data saver, 18 MB/month for Balanced, 80 MB/month for Light live, and 375 MB/month
for Live. These are not carrier billing guarantees: the selected profile and available GPS
fields change payload size, carriers may count protocol overhead, and operating-system,
retry or update traffic is additional.

The selected policy belongs to the hub configuration, not the setup steps. Setup
exchanges the one-time token for a permanent random device credential, stores
it mode `0600`, receives the versioned policy, and invalidates the token. Device
credentials can only use device endpoints.

Use **Configure** on an enrolled tracker to change its cadence later. Saving increments
the configuration version. A reporting tracker notices the new version in its next
successful upload response and replaces its local last-known-good configuration after
validation.

Revoke lost hardware immediately. Rotation returns the replacement credential once and
invalidates the old value. The tracker card reveals that replacement only for the
current operation so it can be copied immediately. Re-enrollment uses a new enrollment
token.

The roster records the self-reported agent version for compatibility diagnostics. The
one-time token and permanent device credential—not catalog membership—control access.
Custom agent authors should use the
[API and conformance guide](../developers/custom-agents.md).
