---
name: hdmi-cec-debug
description: Diagnose the hdmi-cec adapter and the CEC bus through the board's USB console. Use when the adapter is not found, reports an unexpected version, CEC commands end in nack, busy, collision or timeout, or the TV ignores the adapter or the Windows service.
---

# Debug the hdmi-cec adapter

`cec.py` below means `python3 .agents/skills/_shared/cec.py`, run from the repository root.
Talk to the board with `cec.py monitor --command "<console command>"`; each call opens a new
connection, and request state survives between calls. Add `--seconds N` for slow replies and
`--port` when several adapters are connected.

Read [DIAGNOSTICS.md](../../../hdmi-cec/docs/DIAGNOSTICS.md) before interpreting any output:
it defines every console command, result and line reading.

Read-only commands run freely. Commands that transmit or change state (`cec submit`, `send`,
`cec pullup`, `reboot`, `set`, `save`) act on the user's real TV: ask first, naming the effect
(for example, "`4004` turns the TV on").

## Steps

1. **Port.** `hdmi-cec/.venv/bin/python -m serial.tools.list_ports -v` must list one
   `CAFE:4005` device. If it is missing, take the USB items in
   [FLASHING.md troubleshooting](../../../hdmi-cec/docs/FLASHING.md#troubleshooting). If the
   port is busy, ask the user to close the program holding it.
2. **Firmware.** `show version` must print `cec-host-` 0.7.0 or newer. Anything else, or a
   version that differs from what the user expects, is a flashing job: `hdmi-cec-flash` skill.
3. **Line.** `cec line`, then `cec scan` when the line is not idle-high. Interpret it with
   DIAGNOSTICS.md and hand wiring or TV-menu checks to the user.
4. **Frame.** Only with an idle-high line and the user's consent: `cec submit <id> raw <hex>`,
   then `cec status <id>` in a second call until `state=done`.

Done when you can name the cause with the console evidence (command and output) that shows
it, or have handed the user the exact physical check that remains.

When a finding settles a row of [VALIDATION.md](../../../hdmi-cec/docs/VALIDATION.md) or the
"Actual build" table in [HARDWARE.md](../../../hdmi-cec/docs/HARDWARE.md), update that row.

## Windows host

The Windows service runs on another machine. Give the user the commands from
[WINDOWS_SERVICE.md diagnostics](../../../hdmi-cec/docs/WINDOWS_SERVICE.md#diagnostics)
(`hdmi-cec.exe status`, the Event Log query, `hdmi-cec.exe run` for a foreground host), and
read the output they paste back.
