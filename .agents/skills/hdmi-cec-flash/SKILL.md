---
name: hdmi-cec-flash
description: Build and flash the hdmi-cec firmware onto the XIAO RP2350 adapter. Use when asked to flash or re-flash the board, build the firmware, install a released UF2, or recover a board through BOOTSEL.
---

# Flash the hdmi-cec adapter

`cec.py` below means `python3 .agents/skills/_shared/cec.py`, run from the repository root. It
switches into `hdmi-cec/.venv` by itself; `cec.py <command> --help` lists the options.

The board sits on the user's desk. Every physical action (cables, BOOT/RESET buttons) is
theirs: ask with the question tool and wait for the answer before continuing.

## Steps

1. **Tools.** Run `cec.py doctor`; if it fails, run `cec.py setup` (macOS arm64 only:
   Homebrew packages, Arm toolchain, venv, submodules). Done when `doctor` exits 0.
2. **Image.** Pick the source the user asked for:
   - Working tree: `cec.py build`, then `hdmi-cec/.venv/bin/python hdmi-cec/tests/run.py`.
     Image: `hdmi-cec/build/host-rp2350/pico-cec.uf2`.
   - Published release: `cec.py download` (latest) or `cec.py download --tag hdmi-cec-vX.Y.Z`.
     Image: `hdmi-cec/build/releases/<tag>/pico-cec.uf2`.

   Done when the commands exit 0. Note the expected version: `version` in the
   `manifest.json` next to the image.
3. **Flash.** Ask the user to unplug the adapter's HDMI cable.
   - If `cec.py monitor --command "show version"` prints `cec-host-…` or `cec-wake-…`, run
     `cec.py flash --bootsel` (add `--image <uf2>` for a release image).
   - Otherwise, or when that reports `No BOOTSEL device`, ask the user to enter BOOTSEL
     (hold BOOT while plugging in USB, or hold BOOT and tap RESET), then run `cec.py flash`
     (with `--image` as above).
   - `Several boards found` lists `--bus N --address M` pairs: ask the user which one.

   Done when the output ends with `Flash verified and reboot requested` and the newest file
   in `hdmi-cec/build/flash-records/` has `"verified": true`.
4. **Confirm.** Wait a few seconds for USB to re-enumerate, then run
   `cec.py monitor --command "show version"`. Done when it prints exactly the expected
   version from step 2. Then the user can plug HDMI back in.

Report the flashed version and the flash record path.

Failures at any step: [Troubleshooting in FLASHING.md](../../../hdmi-cec/docs/FLASHING.md#troubleshooting).
Line or CEC problems after a good flash belong to the `hdmi-cec-debug` skill.
