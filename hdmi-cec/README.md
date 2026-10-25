# Host-driven HDMI-CEC

Адаптер на Seeed Studio XIAO RP2350 выполняет CEC-команды Windows-хоста. После подачи
питания и reset прошивка `0.7.0` ничего не передаёт в CEC: телевизор включается только
по запросу службы Windows.

## Подключение

| HDMI Type A | XIAO RP2350 |
| --- | --- |
| pin 13, CEC | D10 / GPIO3 |
| pin 17, DDC/CEC GND | GND |

Питание — USB-C. HDMI +5 V, DDC и видеолинии к XIAO не подключаются. Компьютер передаёт
видео отдельным HDMI-кабелем. Подробнее: [docs/HARDWARE.md](docs/HARDWARE.md).

## Прошивка

```sh
cd hdmi-cec
python3 cec.py setup
.venv/bin/python cec.py doctor
.venv/bin/python cec.py build
.venv/bin/python tests/run.py
```

Образ: `build/host-rp2350/pico-cec.uf2`. Запись и восстановление описаны в
[docs/FLASHING.md](docs/FLASHING.md).

USB-протокол прошивки:

```text
cec submit 1 raw 4004      # Image View On -> TV
cec submit 2 raw 4f8220000 # (hex-кадр, 1..16 байт, отправитель всегда 4x)
cec status 2
```

Машинные ответы начинаются с `@cec`. Одновременно выполняется одна команда; прошивка
не делает автоматических повторов и не читает EDID: последовательности кадров собирает
хост-приложение.

## Windows 10/11 x64

Собрать единый EXE на Mac:

```sh
cd hdmi-cec/host
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -o ../build/windows-amd64/hdmi-cec.exe ./cmd/hdmi-cec
```

Установка выполняется в повышенном PowerShell на Windows:

```powershell
.\hdmi-cec.exe install
.\hdmi-cec.exe input HDMI2
.\hdmi-cec.exe status
```

Команды пользователя: `on`, `off`, `input HDMI1`…`HDMI4`, `input 2.1.0.0`, `status`.
Подробная установка и диагностика: [docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md).

Служба включает ТВ при настоящем автоматическом запуске Windows и при
`PBT_APMRESUMEAUTOMATIC`. Через 5 секунд она выбирает сохранённый вход. Ручной запуск
или восстановление службы телевизор не включает; `off` бывает только ручным.

Конфигурация хранится в `%ProgramData%\HdmiCecHost\config.json`. До первой команды
`input` телевизор включается без переключения входа.
