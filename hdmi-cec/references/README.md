# Исходники и воспроизводимость

Основа прошивки: [gkoh/pico-cec](https://github.com/gkoh/pico-cec),
commit `8a5d4690461ae4f676f69511c51063b860789ff7`. Исходники прошивки и список
контрибьюторов находятся в [../firmware/](../firmware/README.md).

Pico SDK, FreeRTOS-Kernel и tcli подключены как git submodules на закреплённых commit
(см. `.gitmodules`, версии каждой сборки записываются в `build/host-rp2350/manifest.json`).
`cec.py setup` инициализирует их, включая TinyUSB из Pico SDK, без обновления на произвольный HEAD.

Полный Arm GNU Toolchain 14.3.rel1 для macOS arm64:
[официальный архив](https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-darwin-arm64-arm-none-eabi.tar.xz).

SHA-256:
`30f4d08b219190a37cded6aa796f4549504902c53cfc3c7e044a8490b6eba1f7`.

Дополнительные источники:

- [pico-cec USB CLI](https://github.com/gkoh/pico-cec/wiki/Command-Line-Interface-Guide).
- [Pico SDK](https://github.com/raspberrypi/pico-sdk).
- [picotool](https://github.com/raspberrypi/picotool).
- [CEC opcodes в Linux](https://github.com/torvalds/linux/blob/master/include/uapi/linux/cec.h).
