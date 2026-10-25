# Windows service

## Requirements

- Windows 10/11 x64.
- Adapter firmware `cec-host-0.7.0` or newer.
- USB CDC device with VID:PID `CAFE:4005`.
- Administrator rights only to install and uninstall the service.

## Installation

1. Download `hdmi-cec.exe` from the
   [latest hdmi-cec release](https://github.com/alexander-litvinovich/entertainment-center/releases)
   and copy it to a permanent folder, e.g. `C:\Program Files\HdmiCecHost\`. Do not move the
   EXE after installation: the service and the updater use this path.
2. Open PowerShell as administrator in that folder.
3. Run `.\hdmi-cec.exe install`.

The command removes the old `HDMI CEC Wake` scheduled task, creates the automatic
`HdmiCecHost` service running as `LocalSystem` with restart-on-failure, registers the Event
Log source and starts the service. This first start has reason `Demand`, so it does not turn
the TV on.

## Running without installing (debugging)

```powershell
.\hdmi-cec.exe run            # host in this window, logs to the console, Ctrl+C exits
.\hdmi-cec.exe run --auto     # same, plus the on -> delay -> input sequence at start
.\hdmi-cec.exe run --config C:\temp\test.json
```

No administrator rights are needed. The host uses the same pipe, so
`.\hdmi-cec.exe on|off|input HDMI2|status` work from another window. The default config is
`%AppData%\HdmiCecHost\config.json`, separate from the service. The pipe is available to the
current user only. If the service is installed and running, it holds the pipe: run
`Stop-Service HdmiCecHost` first.

## Choosing the input and control

Interactive input selection (needs the running service or `run`, and the TV must be on):

```powershell
.\hdmi-cec.exe setup
```

Space switches the TV to the next input, HDMI1…HDMI4; after HDMI4 it beeps and starts again
at HDMI1. Enter saves the current input to the config; Esc or Ctrl+C exits without saving.

Manual selection:

```powershell
.\hdmi-cec.exe input HDMI2
.\hdmi-cec.exe input 2.1.0.0
.\hdmi-cec.exe on
.\hdmi-cec.exe off
.\hdmi-cec.exe status
```

`HDMI1`…`HDMI4` map to `1.0.0.0`…`4.0.0.0`. A raw address has four hex nibbles; after the
first zero only zeros are allowed. `input` saves the choice atomically through the service,
then switches the TV. `on` turns the TV on and applies the saved input 5 seconds later.
`off` cancels a pending input switch and sends Standby. Input switching and `off` first
announce the adapter (Report Physical Address for the saved input), then send Active Source or
Standby.

Any frame can be sent for experiments: `.\hdmi-cec.exe send 4F:82:20:00` (the sender is always
logical address 4, the first byte is `4X`). Check the result with `status`.

The CLI talks only to the local service through `\\.\pipe\hdmi-cec-host`. Remote clients are
rejected; authorised local users can run commands.

## Automatic events

- `Auto` and `DelayedAuto` at Windows boot: `on`, then the input after 5 seconds.
- `PBT_APMRESUMEAUTOMATIC (0x12)`: the same sequence.
- `PBT_APMRESUMESUSPEND`, a manual start and restart-on-failure: no CEC commands.
- Repeated boot/resume events within 30 seconds are coalesced.

The service waits up to 60 seconds for the adapter. Firmware older than `0.7.0` is rejected.
A CEC ACK confirms delivery of the frame, not the actual screen state.

## Updates

The service checks
[GitHub Releases](https://github.com/alexander-litvinovich/entertainment-center/releases)
2 minutes after it starts and then every 24 hours. It takes the highest published
`hdmi-cec-vX.Y.Z` release, downloads `hdmi-cec.exe`, checks it against the release's
`SHA256SUMS`, swaps the EXE in place (the previous one is kept as `hdmi-cec.exe.old` until the
next start) and stops with a failure code, so the SCM restart-on-failure action starts the new
version. That restart sends no CEC commands; a running on → input sequence finishes first.

```powershell
.\hdmi-cec.exe version   # version of this EXE
.\hdmi-cec.exe update    # check and install now, through the service
```

To turn automatic checks off, add `"autoUpdate": false` to
`%ProgramData%\HdmiCecHost\config.json` and restart the service; `update` still works.
Builds without a release version (`version` prints `dev`) never update. Results are written to
the Event Log.

Firmware is not updated automatically; flash it from a Mac
([FLASHING.md](FLASHING.md)).

## Diagnostics

```powershell
.\hdmi-cec.exe status
Get-WinEvent -LogName Application | Where-Object ProviderName -eq HdmiCecHost | Select-Object -First 20
Get-Service HdmiCecHost
```

If the adapter is not found, check Device Manager, the USB data cable, and that no other
program has opened the COM port.

Uninstall from an elevated PowerShell: `.\hdmi-cec.exe uninstall`.
