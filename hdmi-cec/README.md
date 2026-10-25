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

Download `hdmi-cec.exe` from the
[latest hdmi-cec release](https://github.com/alexander-litvinovich/entertainment-center/releases),
or build it on a Mac:

```sh
cd hdmi-cec/host
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-X main.version=$(cat ../VERSION)" -o ../build/windows-amd64/hdmi-cec.exe ./cmd/hdmi-cec
```

Install from an elevated PowerShell on Windows:

```powershell
.\hdmi-cec.exe install
.\hdmi-cec.exe input HDMI2
.\hdmi-cec.exe status
```

User commands: `on`, `off`, `input HDMI1`…`HDMI4`, `input 2.1.0.0`, `status`, `update`,
`version`. Full installation and diagnostics guide:
[docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md).

The service turns the TV on when Windows really starts automatically and on
`PBT_APMRESUMEAUTOMATIC`. Five seconds later it selects the saved input. A manual start or a
service restart does not turn the TV on; `off` is always manual.

The configuration lives in `%ProgramData%\HdmiCecHost\config.json`. Until the first `input`
command the TV turns on without switching inputs.

The service updates itself from GitHub Releases once a day; see
[docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md#updates).

## Releases

[docs/RELEASING.md](docs/RELEASING.md).
