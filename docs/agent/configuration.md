# Agent configuration

The server returns a monotonically versioned configuration during enrollment. The
agent validates a candidate completely before atomically replacing the last-known-good
file. Invalid or older configuration cannot replace a working configuration.

```json
{
  "version": 1,
  "sampling": { "default_seconds": 120, "parked_seconds": 900 },
  "upload": { "default_seconds": 120, "parked_seconds": 900 },
  "vehicle_profile": "citroen-c-zero-v1",
  "vehicle_profile_definition": null
}
```

`version` identifies this tracker's hub-owned configuration revision. Other agent
implementations use the same API described in
[Build a custom agent](../developers/custom-agents.md).

The agent checks movement evidence once per second. This observation is not necessarily a
durable telemetry sample: the example data-saver policy records and immediately attempts
to upload a point every two minutes while driving and every fifteen minutes while parked.
The queue remains authoritative through network loss and deletes only sample IDs
acknowledged by the server.

The Devices page offers data-saver, balanced, light-live and live presets plus two
directly editable fields for the driving and parked intervals. The server requires
sampling and upload to use the same interval in each state. It persists the selected
policy per tracker and increments its configuration version when the policy changes.

A freshly observed `vehicle.ready` or `vehicle.ignition` canonical profile metric takes
priority for driving state, followed by fresh `vehicle.speed` and GPS motion. Without an
explicit off state, zero speed starts a five-minute grace period rather than immediately
declaring the vehicle parked. Legacy configurations without `parked_seconds` continue to
use their default interval for both states.

Every successful telemetry response includes the current server configuration version.
The agent fetches the full configuration immediately when that version is newer and also
performs a six-hour fallback sync. Same-version responses cause no file write. This avoids
downloading an unchanged profile every five minutes on a metered cellular connection. A
syntactically invalid value, rollback, or invalid profile definition is rejected before
replacing the working file. Both bundled and owner-created profiles arrive as a validated
`vehicle_profile_definition` object whose `id` must exactly match `vehicle_profile`; that
definition is persisted in the last-known-good file. The standalone executable therefore
does not need a separately installed profile package.

Using 50 driving hours/month, fifteen-minute parked heartbeats, GPS, the expanded C-Zero
metrics and current JSON/HTTP protocol, planning estimates are about 9 MB/month for Data
saver, 18 MB/month for Balanced, 80 MB/month for Light live and 375 MB/month for Live.
These are not carrier billing guarantees. Payload size depends on the selected profile and
available GPS fields, and a carrier may count protocol overhead. Operating-system traffic,
retries and manual agent updates are outside the telemetry policy.

Inspect the accepted server configuration with `sudo vehinode-agent config`. Hardware is
host-local rather than server configuration: the server cannot reliably know which Linux
serial path belongs to a modem or an OBD adapter. Inspect discovery and the current saved
selection with:

```sh
sudo vehinode-agent devices
```

Each source can be `auto`, `off`, or an explicit path. Save a verified stable choice and
restart the service with:

```sh
sudo vehinode-agent devices set \
  --gps /dev/serial/by-id/usb-SimTech_SIM7600... \
  --obd /dev/serial/by-id/usb-OBDLink_SX...
sudo systemctl restart vehinode-agent
```

Use `--gps off` or `--obd off` when the hardware is intentionally absent. `auto` prefers
recognizable USB identities and otherwise presents conventional `ttyUSB`/`ttyACM`
candidates. Always prefer `/dev/serial/by-id/...` over changing `/dev/ttyUSB0` numbers.
The low-level `run --gps-device` and `run --obd-device` options remain temporary runtime
overrides; normal installations should persist choices through `devices set`.
