# hdmi-cec firmware

Firmware for an HDMI-CEC USB adapter on the Seeed Studio XIAO RP2350. It is passive: it never
sends anything on its own. Over the USB console it accepts CEC frames from the host application
and reports the transmission result (`ack`, `nack`, `busy`, `timeout`, `collision`, `error`).

The firmware is built on [pico-cec](https://github.com/gkoh/pico-cec) by Guo-Rong Koh (MIT
license), commit
[`8a5d469`](https://github.com/gkoh/pico-cec/commit/8a5d4690461ae4f676f69511c51063b860789ff7).
The original license is kept in [LICENSE](LICENSE).

## pico-cec contributors

Taken from the git history of the original repository at commit `8a5d469`.

- Guo-Rong Koh ([@gkoh](https://github.com/gkoh)), project author
- Ansuz Inspirati
- Flaviu Tamas
- Michał Budzyński
- Alex Martens
- Kenny Jackson
- christian-0s
- [@dennizzzz](https://github.com/dennizzzz)

The changes made in this project (host-driven mode, the `cec submit` USB protocol, line
diagnostics) are by Alexander Litvinovich.

## Changes from pico-cec

- The standalone mode is gone: EDID reading, physical address detection, logical address
  allocation, answering TV requests and sending Active Source. What remains is a single command
  executor (`src/cec-command.c`, `src/cec-command-task.c`) with a one-command queue.
- New machine interface on the USB console: `cec submit <id> raw <hex>`, `cec status <id>`.
  Frames are always sent from logical address 4.
- The frame transmitter (`src/cec-frame.c`) was rewritten: it returns the transmission result,
  checks arbitration on every `1` bit, and no longer hangs on a busy line.
- Diagnostics: `cec line` (CEC line level over 200 ms), `cec scan` (which board pins read high),
  `cec pullup on|off` (internal pull-up on the pin until reset).
- The board LED is kept off and the frame log is reduced to a hex dump.
- Removed what this build does not use: the printable enclosure (OpenSCAD), RP2040 boards, CI and
  the original project's documentation.

## Layout

| Path | Purpose |
| --- | --- |
| `src/`, `include/` | Firmware sources |
| `crc/` | CRC library ([gityf/crc](https://github.com/gityf/crc)) |
| `CMakeLists.txt` | Build |
| `pico-sdk/`, `FreeRTOS-Kernel/`, `tcli/` | Git submodules: [Pico SDK](https://github.com/raspberrypi/pico-sdk), [FreeRTOS-Kernel](https://github.com/raspberrypi/FreeRTOS-Kernel), [tcli](https://github.com/dpse/tcli) |

## Building

From the repository root: `python3 .agents/skills/_shared/cec.py setup`, then
`python3 .agents/skills/_shared/cec.py build`. Board and pin settings live in
[config.cmake](../config.cmake); the version lives in [VERSION](../VERSION). Flashing is
described in [docs/FLASHING.md](../docs/FLASHING.md).
