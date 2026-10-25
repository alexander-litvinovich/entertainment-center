# Sources and reproducibility

The firmware is based on [gkoh/pico-cec](https://github.com/gkoh/pico-cec),
commit `8a5d4690461ae4f676f69511c51063b860789ff7`. The firmware sources and the list of
contributors are in [../firmware/](../firmware/README.md).

Pico SDK, FreeRTOS-Kernel and tcli are git submodules pinned to fixed commits
(see `.gitmodules`; the versions used by each build are recorded in
`build/host-rp2350/manifest.json`). `cec.py setup` initialises them, including TinyUSB from
the Pico SDK, without moving them to an arbitrary HEAD.

Full Arm GNU Toolchain 14.3.rel1 for macOS arm64:
[official archive](https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-darwin-arm64-arm-none-eabi.tar.xz).

SHA-256:
`30f4d08b219190a37cded6aa796f4549504902c53cfc3c7e044a8490b6eba1f7`.

Further sources:

- [pico-cec USB CLI](https://github.com/gkoh/pico-cec/wiki/Command-Line-Interface-Guide).
- [Pico SDK](https://github.com/raspberrypi/pico-sdk).
- [picotool](https://github.com/raspberrypi/picotool).
- [CEC opcodes in Linux](https://github.com/torvalds/linux/blob/master/include/uapi/linux/cec.h).
