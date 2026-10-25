# Flashing the XIAO RP2350

Run every command from the repository root. `cec.py` below means
`python3 .agents/skills/_shared/cec.py`.

Disconnect HDMI while flashing. Connect the XIAO to the Mac with a USB data cable.

## 1. Get an image

Build it locally:

```sh
python3 .agents/skills/_shared/cec.py setup     # once: toolchain, venv, submodules
python3 .agents/skills/_shared/cec.py build
hdmi-cec/.venv/bin/python hdmi-cec/tests/run.py
```

This produces `hdmi-cec/build/host-rp2350/pico-cec.uf2`, `.elf` and `manifest.json`.
`picotool info -a hdmi-cec/build/host-rp2350/pico-cec.uf2` must show RP2350,
`seeed_xiao_rp2350` and ARM Secure.

Or download a published release (UF2 and manifest, checked against `SHA256SUMS`):

```sh
python3 .agents/skills/_shared/cec.py download                       # latest
python3 .agents/skills/_shared/cec.py download --tag hdmi-cec-v0.7.0
```

Files land in `hdmi-cec/build/releases/<tag>/`; pass that UF2 to `flash --image`.

## 2. First flash through BOOTSEL

1. Unplug USB.
2. Hold BOOT, plug in USB, release BOOT.
3. Alternative with the board connected: hold BOOT, press and release RESET, then release BOOT.
4. Run `picotool info -a` and check that an RP2350 is detected.
5. Flash and verify:

   ```sh
   python3 .agents/skills/_shared/cec.py flash
   python3 .agents/skills/_shared/cec.py flash --image hdmi-cec/build/releases/<tag>/pico-cec.uf2
   ```

With several boards in BOOTSEL the tool prints the choices; repeat with `--bus N --address M`.
Each attempt writes the version, SHA-256 and verification result to
`hdmi-cec/build/flash-records/`.

## 3. Re-flashing a running board

When the USB console works, the tool reboots the board into BOOTSEL itself:

```sh
python3 .agents/skills/_shared/cec.py flash --bootsel
```

With several CDC devices add `--port /dev/cu.usbmodem…`. If the board does not enter the
bootloader, use BOOT/RESET from the previous section.

## 4. Check the passive start

```sh
python3 .agents/skills/_shared/cec.py monitor --command "show version"
python3 .agents/skills/_shared/cec.py monitor --command "cec status 1"
```

The version must match `version` in the image's `manifest.json` (`cec-host-<VERSION>-<hash>`).
After reset the board only initialises USB and CEC: it sends nothing by itself.

Console commands and CEC line diagnostics: [DIAGNOSTICS.md](DIAGNOSTICS.md).

## 5. Manual UF2 copy

1. Put the board into BOOTSEL.
2. Check it with `picotool info -a`.
3. Copy `pico-cec.uf2` to the `RP2350` drive.
4. Wait for the drive to disappear and the USB console to appear.
5. Optionally re-enter BOOTSEL and run:

   ```sh
   picotool verify hdmi-cec/build/host-rp2350/pico-cec.uf2
   picotool reboot
   ```

## Troubleshooting

- **No RP2350 drive.** Repeat BOOT/RESET, use a data cable, connect the board directly
  without a hub.
- **Console not found.** Close programs holding the port and run
  `hdmi-cec/.venv/bin/python -m serial.tools.list_ports -v`. The adapter is VID:PID `CAFE:4005`.
- **The TV turns on at power-up after flashing.** Check `show version`: the board still runs the
  old `cec-wake-0.5.0`; flash `cec-host` again.
- **A CEC command has no effect.** Check HDMI pins 13/17, CEC in the TV menu and the
  `cec status` result. `nack` and `busy` are bus results, not confirmation of the screen state.
