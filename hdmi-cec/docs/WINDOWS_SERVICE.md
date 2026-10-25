# Windows service

## Requirements

- Windows 10/11 x64.
- Adapter firmware `cec-host-0.7.0` or newer.
- USB CDC device with VID:PID `CAFE:4005`.
- Administrator rights only to install and uninstall the service.

## Installation

Run in PowerShell (a normal window is fine: the script asks for administrator rights through
UAC and continues in an elevated window):

```powershell
irm https://raw.githubusercontent.com/alexander-litvinovich/entertainment-center/master/hdmi-cec/install.ps1 | iex
```

[install.ps1](../install.ps1):

1. finds the highest published `hdmi-cec-vX.Y.Z` release, downloads `hdmi-cec.exe` and checks
   it against the release's `SHA256SUMS`;
2. stops the `HdmiCecHost` service if it runs and copies the EXE to
   `%ProgramFiles%\HdmiCecHost\hdmi-cec.exe`;
3. runs `hdmi-cec.exe install` and waits for the service to start;
4. adds `%ProgramFiles%\HdmiCecHost` to the machine `PATH`, so `hdmi-cec` works in new terminals;
5. opens `hdmi-cec setup` when `%ProgramData%\HdmiCecHost\config.json` does not exist yet.

Run the same command again to update or repair an installation; the settings are kept.

### Manual installation

1. Download `hdmi-cec.exe` from the
   [latest hdmi-cec release](https://github.com/alexander-litvinovich/entertainment-center/releases)
   and copy it to a permanent folder, e.g. `C:\Program Files\HdmiCecHost\`. The service and
   the updater use this path.
2. Open PowerShell as administrator in that folder.
3. Run `.\hdmi-cec.exe install`, then `.\hdmi-cec.exe setup`.

`install` removes the old `HDMI CEC Wake` scheduled task, creates the automatic `HdmiCecHost`
service running as `LocalSystem` with restart-on-failure, registers the Event Log source and
starts the service. This first start has reason `Demand`, so it does not turn the TV on. If
the service already exists, `install` points it at this EXE and starts it if it is stopped.

## Setup

```powershell
hdmi-cec setup
```

Setup needs the running service (or `hdmi-cec run`) and an interactive console.

Without a saved input, setup starts with onboarding: enter the input number of the TV (or
type 1-4 right away), or detect it. Detection explains itself on screen: press O to send Power
On if the TV is off, then Space switches the TV to HDMI1, HDMI2, … (a beep after HDMI4 marks
the start of the list again), and Enter takes the input the TV shows. Nothing is saved until
Save in the settings form:

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

| Key | Action |
| --- | --- |
| ↑ ↓ Tab | move between fields |
| ← → or 1-4 | change the video input; ← → also move between Save and Cancel |
| Space, Enter | toggle a checkbox, press a button |
| Esc, Ctrl+C | cancel without saving |

Save writes the whole configuration through the service; it sends no CEC frames. An input
that is not `N.0.0.0` (for example `2.1.0.0`, saved with `input`) is shown as is and kept
unless you pick another one.

## Running without installing (debugging)

```powershell
.\hdmi-cec.exe run              # host in this window, logs to the console, Ctrl+C exits
.\hdmi-cec.exe run --auto       # same, plus the on -> delay -> input sequence at start
.\hdmi-cec.exe run --dry-run    # no adapter: log CEC frames instead of sending them
.\hdmi-cec.exe run --config C:\temp\test.json
.\hdmi-cec.exe setup --dry-run  # setup screens alone, against a local config, no host needed
```

No administrator rights are needed. The host uses the same pipe, so `setup`, `on`, `off`,
`input HDMI2` and `status` work from another window. The default config is
`%AppData%\HdmiCecHost\config.json`, separate from the service. The pipe is available to the
current user only. If the service is installed and running, it holds the pipe: run
`Stop-Service HdmiCecHost` first.

The same commands run on macOS and Linux for development; there the host listens on a Unix
socket (`$TMPDIR/hdmi-cec-host.sock`, mode 0600) instead of the named pipe.

## Control from the command line

```powershell
hdmi-cec input HDMI2
hdmi-cec input 2.1.0.0
hdmi-cec on
hdmi-cec off
hdmi-cec status
```

`HDMI1`…`HDMI4` map to `1.0.0.0`…`4.0.0.0`. A raw address has four hex nibbles; after the
first zero only zeros are allowed. `input` saves the choice atomically through the service,
then switches the TV. `on` turns the TV on and applies the saved input 5 seconds later.
`off` cancels a pending input switch and sends Standby. Input switching and `off` first
announce the adapter (Report Physical Address for the saved input), then send Active Source or
Standby.

Any frame can be sent for experiments: `hdmi-cec send 4F:82:20:00` (the sender is always
logical address 4, the first byte is `4X`). Check the result with `status`.

The CLI talks only to the local service through `\\.\pipe\hdmi-cec-host`. Remote clients are
rejected; authorised local users can run commands and change the settings.

## Automatic events

| Setup option | Windows event | CEC |
| --- | --- | --- |
| Turn TV on: Startup | service start reason `Auto` or `DelayedAuto` at boot | on, then the input after 5 s |
| Turn TV on: Wake | `PBT_APMRESUMEAUTOMATIC (0x12)` | on, then the input after 5 s |
| Turn TV off: Sleep | `PBT_APMSUSPEND (0x4)` | report address, Standby |
| Turn TV off: Shutdown | `SERVICE_CONTROL_PRESHUTDOWN` (shutdown and restart) | report address, Standby |

- `PBT_APMRESUMESUSPEND`, a manual start or stop, and restart-on-failure send no CEC commands.
- Repeated boot/resume events within 30 seconds are coalesced.
- Standby on sleep is best effort: Windows does not wait for services before it suspends. At
  shutdown the service stops only after Standby is sent (at most 5 seconds).
- With Fast Startup enabled, Windows hibernates instead of shutting down, so the service may
  see shutdown and boot as sleep and wake.
- A fresh install enables all four. A config written by 0.7.0 or earlier keeps its old
  behaviour (Wake and Startup on, Sleep and Shutdown off) until the next Save in setup.

In `config.json`:

```json
{
  "input": "2.0.0.0",
  "inputDelayMs": 5000,
  "turnOn": { "wake": true, "startup": true },
  "turnOff": { "sleep": true, "shutdown": true }
}
```

The service reads the file at start; `setup` and `input` change it through the running service.

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
hdmi-cec version   # version of this EXE
hdmi-cec update    # check and install now, through the service
```

To turn automatic checks off, add `"autoUpdate": false` to
`%ProgramData%\HdmiCecHost\config.json` and restart the service; `update` still works.
Builds without a release version (`version` prints `dev`) never update. Results are written to
the Event Log.

Firmware is not updated automatically; flash it from a Mac
([FLASHING.md](FLASHING.md)).

## Diagnostics

```powershell
hdmi-cec status
Get-WinEvent -LogName Application | Where-Object ProviderName -eq HdmiCecHost | Select-Object -First 20
Get-Service HdmiCecHost
```

If the adapter is not found, check Device Manager, the USB data cable, and that no other
program has opened the COM port.

Uninstall from an elevated PowerShell: `hdmi-cec uninstall`.
