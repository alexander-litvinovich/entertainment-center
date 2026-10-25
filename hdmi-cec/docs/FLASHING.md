# Запись прошивки 0.6.0 на XIAO RP2350

## 1. Собрать образ

На время записи отключите HDMI. Подключите XIAO к Mac USB-кабелем с передачей данных.

```sh
cd hdmi-cec
python3 cec.py setup
.venv/bin/python cec.py doctor
.venv/bin/python cec.py build
.venv/bin/python tests/run.py
```

Должны появиться `build/host-rp2350/pico-cec.uf2`, `.elf` и `manifest.json`.
Проверка `picotool info -a build/host-rp2350/pico-cec.uf2` должна показать RP2350,
`seeed_xiao_rp2350` и ARM Secure.

## 2. Первая запись через BOOTSEL

1. Отключите USB.
2. Удерживая BOOT, подключите USB и отпустите BOOT.
3. Альтернатива при подключённой плате: удерживайте BOOT, нажмите и отпустите RESET,
   затем отпустите BOOT.
4. Выполните `picotool info -a` и убедитесь, что обнаружен RP2350.
5. Запишите и проверьте образ:

   ```sh
   .venv/bin/python cec.py flash
   ```

Если подключено несколько загрузчиков, утилита напечатает варианты; повторите команду
с `--bus N --address M`. Журнал версии и SHA-256 сохраняется в `build/flash-records/`.

## 3. Последующие записи

Когда USB-консоль работает:

```sh
.venv/bin/python cec.py build
.venv/bin/python cec.py flash --bootsel
```

При нескольких CDC-устройствах добавьте `--port /dev/cu.usbmodem…`. Если переход в
загрузчик не сработал, используйте BOOT/RESET из предыдущего раздела.

## 4. Проверить пассивный запуск

```sh
.venv/bin/python cec.py monitor --command "show version"
.venv/bin/python cec.py monitor --command "cec status 1"
```

Версия должна начинаться с `cec-host-0.6.0`. После reset плата только инициализирует
USB и CEC: она не включает ТВ, не выбирает вход и не передаёт Active Source.

Ручная проверка протокола:

```text
cec submit 1 on
cec status 1
cec submit 2 input 2000
cec status 2
cec submit 3 off
cec status 3
```

Диагностика линии CEC: `cec line` измеряет её 200 мс. `high=2000/2000` — линия подтянута ТВ;
`high=0/2000` — подтяжки нет, команды завершатся `busy`/`collision`. Для эксперимента
`cec pullup on|off` включает внутреннюю подтяжку пина до сброса; результат `nack` значит,
что на шине никто не подтверждает кадры.

Ответы имеют вид `@cec id=1 state=queued`, затем
`@cec id=1 state=done result=ack|nack|busy|timeout|collision|error`.

## 5. Ручная запись UF2

1. Введите плату в BOOTSEL.
2. Проверьте её через `picotool info -a`.
3. Скопируйте `build/host-rp2350/pico-cec.uf2` на диск `RP2350`.
4. Дождитесь исчезновения диска и появления USB-консоли.
5. При необходимости повторно войдите в BOOTSEL и выполните:

   ```sh
   picotool verify build/host-rp2350/pico-cec.uf2
   picotool reboot
   ```

## Если что-то не работает

- **Нет диска RP2350.** Повторите BOOT/RESET, замените кабель на кабель с данными,
  подключите плату напрямую без хаба.
- **Консоль не найдена.** Закройте программы, занявшие порт, и выполните
  `.venv/bin/python -m serial.tools.list_ports -v`.
- **После записи ТВ включается от питания.** Проверьте `show version`: на плате осталась
  старая `cec-wake-0.5.0`, повторите запись `cec-host-0.6.0`.
- **CEC-команда не действует.** Проверьте HDMI pins 13/17, CEC в меню ТВ и результат
  `cec status`. `nack` и `busy` — результат шины, а не подтверждение состояния экрана.
