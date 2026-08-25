# Citroën C-Zero profile

`citroen-c-zero-v1` covers the Mitsubishi i-MiEV / Peugeot iOn family using a safe,
declarative decoder. It includes community-documented battery, cell, charge, temperature,
speed and odometer mappings plus operator-supplied READY, motor, range, door/light,
warning, parking-brake and per-wheel pressure/temperature mappings. CAN IDs include
`0x101`, `0x286`, `0x298`, `0x346`, `0x373`, `0x374`, `0x384`, `0x389`, `0x3D3`,
`0x412` and `0x424`.

Previously sourced signals remain marked **experimental** and newly transcribed signals
are marked **unknown**. Every formula has a synthetic decoder fixture, but none of those
labels claims validation on this project's physical C-Zero, OBDLink SX and wiring. Values
outside declared sanity ranges are discarded.

The `0x101` mapping produces generic `vehicle.ready` and `vehicle.operating_state`
metrics. A fresh READY value takes priority over GPS in the agent's parked-state detector.
The supplied PHP also contained an active `0x761`/`0x762` diagnostic capacity request;
that request is intentionally excluded because vehicle profiles are passive decoders.
The `0x373` current decoder follows the supplied PHP's amps-into-pack formula, making
charging/regeneration positive and discharge negative. It uses the uncalibrated `32768`
zero-current midpoint; a physical charge/discharge capture is still required before
applying any vehicle-specific sensor offset.

The profile records source URLs and notes. It is an independent YAML implementation;
no third-party source code is copied. Relevant research includes
[bonybrown/iMiev Hacking Tools](https://github.com/bonybrown/iMiev-Hacking-Tools) and
[plaes/i-miev-obd2](https://github.com/plaes/i-miev-obd2). Community documentation is
evidence, not proof of correctness for every model year.

Before changing a formula, capture a reproducible drive/charge trace, corroborate the
physical measurement, update references/status, and add a fixture regression test.
