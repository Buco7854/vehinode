# Agent installation

The tracker is a self-contained Go executable for Linux and runs directly under
systemd. It does not require Docker, Node, Python, a virtual environment or an OS
package-manager update. In **Devices**, create an enrollment token and copy the
generated one-time command:

```sh
curl -fsSL https://vehinode.example/install-agent \
  | sudo sh -s -- --server https://vehinode.example --token ONE_TIME_TOKEN --version 0.1.0
```

The bootstrap validates Linux, systemd and the server URL, detects the CPU, then
downloads exactly one executable and its SHA-256 file. Published targets are
`linux-armv6`, `linux-armv7`, `linux-arm64` and `linux-amd64`; a machine receives only
the matching artifact. The executable creates the unprivileged `vehinode-agent`
account with serial-port access, enrolls, installs its systemd unit, and runs
diagnostics. The enrollment token expires and is consumed once; the permanent device
credential is returned only to the installer and stored mode-restricted in
`/etc/vehinode-agent`.

When the configured public URL is plain HTTP on a private development network, the UI
adds `--allow-insecure-http` to make that risk explicit. The choice is stored with the
device credentials so enrollment, uploads, configuration and updates behave
consistently. Use HTTPS for production because enrollment credentials and telemetry are
otherwise visible on the network.

The installer is idempotent for its directories, account and service. To upgrade to a
published version:

```sh
sudo vehinode-agent update --version 0.1.1
```

All four artifacts are built from the same Go source with CGO disabled. SQLite, HTTP,
profile decoding and serial support are compiled into the executable. There is no
database daemon or shared runtime on the tracker. Different CPU instruction sets still
require different machine-code files, which is why the bootstrap performs architecture
detection instead of pretending that one physical file can run on every processor.

The executable can run manually on other Linux systems. The automatic lifecycle expects
systemd and the standard `useradd`, `groupadd`, `usermod`, `curl`, `sha256sum`, and
`install` commands.

To fully remove the service, binary, permanent device credential, saved hardware choices
and queued telemetry, run:

```sh
sudo vehinode-agent uninstall
```

The command requires typing `uninstall`. Automation can pass `--yes`. Shared
operating-system files are untouched. This operation cannot be undone, and an erased
device credential requires a new one-time enrollment token before reinstalling.

Raspberry Pi OS Stretch is retired and no longer receives security maintenance. The
standalone ARMv6 artifact avoids its broken APT repository and can be used for isolated
local testing, but the installer prints an explicit warning. Re-image with a current
Raspberry Pi OS release before using the tracker with real credentials or outside a
trusted test network.

Never modify the command to install a branch or an unversioned “latest” build.

The Go executable is the bundled implementation, not a server requirement. **Custom
agent** exposes the server URL, one-time token, minimum enrollment fields and API
reference for collectors written in another language or targeting other hardware. An
agent needs no catalog entry or agent ID to enroll. See
[Build a custom agent](../developers/custom-agents.md).
