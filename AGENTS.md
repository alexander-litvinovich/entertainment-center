# entertainment-center

Living-room PC setup. The one project so far is [hdmi-cec](hdmi-cec/README.md): a Seeed XIAO
RP2350 HDMI-CEC adapter (C firmware on Pico SDK + FreeRTOS) driven by a Windows service and
CLI (Go).

## Layout

- `hdmi-cec/firmware/`: a fork of [gkoh/pico-cec](https://github.com/gkoh/pico-cec). Keep its
  `LICENSE` and the contributor list in `firmware/README.md`. `pico-sdk`, `FreeRTOS-Kernel` and
  `tcli` are submodules pinned to the commits recorded in git.
- `hdmi-cec/host/`: Go host. `internal/core` is platform-neutral and tested on macOS;
  Windows-only code lives in `internal/winhost` and `*_windows.go` files.
- `hdmi-cec/VERSION`: the single version for the firmware, the host EXE and the release tag
  `hdmi-cec-vX.Y.Z`.
- `.agents/skills/`: agent skills. `_shared/cec.py` is the build, flash and console tool for
  every skill and for CI; new flashing or debugging tooling goes into `_shared/` or the owning
  skill's folder.

## Rules

- Write all documentation, comments, commit messages and user-facing strings in English.
- The firmware is passive: it transmits only frames the host submits, always from logical
  address 4. CEC sequences (on, input, standby) belong in `hdmi-cec/host/internal/core`.
- A change to the USB protocol updates the host's minimum firmware check
  (`checkVersion` in `hdmi-cec/host/internal/device/serial.go`) in the same change.
- The board and TV are real hardware: flash or transmit CEC frames when the user asked for it,
  and ask first otherwise.

## Checks

- `hdmi-cec/.venv/bin/python hdmi-cec/tests/run.py`: firmware unit tests and tooling tests.
- `cd hdmi-cec/host && go test ./... && GOOS=windows go vet ./...`
- The firmware compiles only through `python3 .agents/skills/_shared/cec.py build`.

## Docs

- Releasing: [hdmi-cec/docs/RELEASING.md](hdmi-cec/docs/RELEASING.md).
- Windows service and self-update: [hdmi-cec/docs/WINDOWS_SERVICE.md](hdmi-cec/docs/WINDOWS_SERVICE.md).
- Wiring: [hdmi-cec/docs/HARDWARE.md](hdmi-cec/docs/HARDWARE.md).
