# Исходники и воспроизводимость

Основа: [gkoh/pico-cec](https://github.com/gkoh/pico-cec),
commit `8a5d4690461ae4f676f69511c51063b860789ff7`.
`cec.py setup` клонирует их в `../firmware/` (каталог не входит в репозиторий); MIT-лицензия upstream остаётся в этом checkout.

Список закреплённых submodules находится в `upstream-lock.json` и manifest каждой сборки.
Зависимости восстанавливаются по gitlinks, без обновления на произвольный HEAD.

Наши изменения хранятся только в `../patches/host-driven.patch`.
`cec.py setup` восстанавливает checkout по закреплённому commit и применяет patch.
При несовместимых локальных изменениях останавливается, не сбрасывая их.
После редактирования прошивки обновите patch:

```sh
git -C firmware add -N include/cec-command.h src/cec-command.c src/cec-command-task.c
git -C firmware diff > patches/host-driven.patch
```

Полный Arm GNU Toolchain 14.3.rel1 для macOS arm64:
[официальный архив](https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-darwin-arm64-arm-none-eabi.tar.xz).

SHA-256:
`30f4d08b219190a37cded6aa796f4549504902c53cfc3c7e044a8490b6eba1f7`.

Дополнительные источники:

- [pico-cec USB CLI](https://github.com/gkoh/pico-cec/wiki/Command-Line-Interface-Guide).
- [Pico SDK](https://github.com/raspberrypi/pico-sdk).
- [picotool](https://github.com/raspberrypi/picotool).
- [CEC opcodes в Linux](https://github.com/torvalds/linux/blob/master/include/uapi/linux/cec.h).
