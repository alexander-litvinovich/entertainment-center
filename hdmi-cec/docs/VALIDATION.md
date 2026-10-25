# Validation of the host-driven version 0.7.0

## Automated

- Frames: `on = 40:04`, `off = 40:36`, Report Physical Address `4F:84:hh:ll:04`, Active Source `4F:82:hh:ll`.
- Validation of HDMI1–HDMI4 and nested physical addresses.
- ACK/NACK/busy/timeout/collision and CEC line release in the driver model.
- The automatic sequence, event coalescing and cancelling a pending input with `off`.
- Atomic input saving; version detection and machine responses with request IDs.
- Self-update: release selection, SHA-256 check, binary swap.
- Config defaults and legacy configs, the turn on/off options per power event, saving the
  whole config through the host.
- Setup screens (onboarding, detection, settings form) driven by key presses against a dry-run
  controller; the CLI, `run --dry-run` and `setup` checked end to end on macOS.
- `install.ps1`: parses in PowerShell 7; release lookup, download and checksum checked against
  the published 0.7.0 release.
- Native Go tests and the `windows/amd64` cross-build.
- UF2 build for `seeed_xiao_rp2350`.

## On the device and Windows

| Check | Result |
| --- | --- |
| Flash and read back `cec-host-0.7.0` | Passed: `show version`; `cec submit`/`status` parse commands, bad arguments give `rejected` |
| Power-up does not turn the TV on | Pending |
| `cec submit … on/off` reaches the TV | `collision` when tested from the Mac; cause unknown (check HDMI pins 13/17 and the CEC pull-up) |
| `hdmi-cec on` turns the TV on | Pending Windows |
| The saved input is applied after 5 s | Pending Windows |
| Restarting the service sends nothing | Pending Windows |
| Resume runs one sequence | Pending Windows |
| `hdmi-cec off` turns the TV off | Pending Windows |
| The CLI works for a regular local user | Pending Windows |
| Self-update installs a newer release and the service restarts without CEC traffic | Pending Windows |
| `install.ps1` from a non-elevated PowerShell: UAC, install, PATH, onboarding | Pending Windows |
| `install.ps1` over an existing installation keeps the config and restarts the service | Pending Windows |
| `hdmi-cec setup` detection switches the TV and saves the chosen input | Pending Windows |
| Sleep sends Standby before the PC suspends | Pending Windows |
| Shutdown and restart send Standby | Pending Windows |

The user confirmed earlier that this adapter and TV react to a single `Image View On` at
power-up with the old firmware. The TV model and HDMI port are not recorded yet.
