# Host-driven HDMI-CEC

A Seeed Studio XIAO RP2350 adapter executes CEC commands for a Windows host. After power-up
or reset the firmware sends nothing on the CEC bus: the TV turns on only when the Windows
service asks for it.

## Wiring

| HDMI Type A | XIAO RP2350 |
| --- | --- |
| pin 13, CEC | D10 / GPIO3 |
| pin 17, DDC/CEC GND | GND |

Power comes from USB-C. HDMI +5 V, DDC and the video lines are not connected to the XIAO. The
PC sends video over a separate HDMI cable. Details: [docs/HARDWARE.md](docs/HARDWARE.md).

## Firmware

Run from the repository root (the tooling lives in
[.agents/skills/_shared/](../.agents/skills/_shared/cec.py) and re-runs itself inside
`hdmi-cec/.venv` once `setup` has created it):

```sh
python3 .agents/skills/_shared/cec.py setup
python3 .agents/skills/_shared/cec.py doctor
python3 .agents/skills/_shared/cec.py build
hdmi-cec/.venv/bin/python hdmi-cec/tests/run.py
```

Image: `hdmi-cec/build/host-rp2350/pico-cec.uf2`. Flashing and recovery:
[docs/FLASHING.md](docs/FLASHING.md). USB console commands and CEC line diagnostics:
[docs/DIAGNOSTICS.md](docs/DIAGNOSTICS.md).

USB protocol:

```text
cec submit 1 raw 4004      # Image View On -> TV
cec submit 2 raw 4f822000  # Active Source (hex frame of 1..16 bytes, sender 4x)
cec status 2
```

Machine responses start with `@cec`. One command runs at a time. The firmware never retries
and never reads EDID: the host application builds the frame sequences.

## Windows 10/11 x64

Install or update with one command in PowerShell (it asks for administrator rights):

```powershell
irm https://raw.githubusercontent.com/alexander-litvinovich/entertainment-center/master/hdmi-cec/install.ps1 | iex
```

[install.ps1](install.ps1) downloads `hdmi-cec.exe` from the
[latest hdmi-cec release](https://github.com/alexander-litvinovich/entertainment-center/releases),
checks it against `SHA256SUMS`, installs it to `%ProgramFiles%\HdmiCecHost` (added to `PATH`),
starts the `HdmiCecHost` service and, on a first install, opens the setup:

```text
HDMI-CEC Setup

Video input: < 2 >

Turn TV on:
[x] Wake
[x] Startup

Turn TV off:
[x] Sleep
[x] Shutdown

Save   Cancel
```

Without a saved input the setup first asks for the TV input number, or detects it by
switching the TV through HDMI1…HDMI4. Open it again any time with `hdmi-cec setup`.

User commands: `setup`, `on`, `off`, `input HDMI1`…`HDMI4`, `input 2.1.0.0`, `status`,
`update` (`--force` reinstalls the latest release), `version`. Full installation and diagnostics guide:
[docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md).

The service turns the TV on when Windows starts or wakes from sleep, selects the saved input
five seconds later, and puts the TV into standby when Windows sleeps or shuts down; each
event can be switched off in the setup. A manual start or a service restart does not turn the
TV on.

The configuration lives in `%ProgramData%\HdmiCecHost\config.json`. The service updates
itself from GitHub Releases once a day; see
[docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md#updates).

### Development and dry run

The host builds and runs on macOS and Linux too. `--dry-run` replaces the adapter with a
simulation that logs every CEC frame, so the CLI, the setup screens and the host logic can be
tried on any machine:

```sh
cd hdmi-cec/host
go test ./...
go run ./cmd/hdmi-cec setup --dry-run   # setup screens against a local config, no host needed
go run ./cmd/hdmi-cec run --dry-run     # host in this terminal; in another one:
go run ./cmd/hdmi-cec setup             #   setup, on, off, input HDMI2, status, …
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-X main.version=$(cat ../VERSION)" -o ../build/windows-amd64/hdmi-cec.exe ./cmd/hdmi-cec
```

Outside Windows the host listens on a Unix socket in the temp directory instead of the named
pipe; `install`, `uninstall` and `service` are Windows-only.

## Releases

[docs/RELEASING.md](docs/RELEASING.md).
