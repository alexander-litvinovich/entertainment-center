# Прошивка hdmi-cec

Прошивка USB-адаптера HDMI-CEC на Seeed Studio XIAO RP2350. Она пассивна: сама ничего не
отправляет, а по USB-консоли принимает от хост-приложения CEC-кадры и возвращает результат
передачи (`ack`, `nack`, `busy`, `timeout`, `collision`, `error`).

Прошивка построена на базе проекта [pico-cec](https://github.com/gkoh/pico-cec)
(Guo-Rong Koh, лицензия MIT), коммит
[`8a5d469`](https://github.com/gkoh/pico-cec/commit/8a5d4690461ae4f676f69511c51063b860789ff7).
Исходная лицензия сохранена в [LICENSE](LICENSE).

## Контрибьюторы pico-cec

Список составлен по истории git оригинального репозитория на коммит `8a5d469`
(число коммитов в скобках).

- Guo-Rong Koh ([@gkoh](https://github.com/gkoh)), автор проекта (96)
- Ansuz Inspirati (3)
- Flaviu Tamas (2)
- Michał Budzyński (2)
- Alex Martens (1)
- Kenny Jackson (1)
- christian-0s (1)
- [@dennizzzz](https://github.com/dennizzzz) (1)

Изменения этого проекта (хост-управляемый режим, USB-протокол `cec submit`, диагностика линии)
внёс Alexander Litvinovich.

## Что изменено относительно pico-cec

- Убран автономный режим: чтение EDID, определение физического адреса, занятие логического
  адреса, ответы на запросы ТВ и отправка Active Source. Остался один исполнитель команд
  (`src/cec-command.c`, `src/cec-command-task.c`) с очередью из одной команды.
- Новый машинный интерфейс USB-консоли: `cec submit <id> raw <hex>`, `cec status <id>`.
  Отправитель кадра всегда логический адрес 4.
- Переписан передатчик кадров (`src/cec-frame.c`): возвращает результат передачи, проверяет
  арбитраж на каждом бите `1`, не зависает на занятой линии.
- Диагностика: `cec line` (уровень линии CEC за 200 мс), `cec scan` (какие выводы платы читаются
  высокими), `cec pullup on|off` (внутренняя подтяжка пина до сброса).
- Светодиод платы погашен, журнал кадров сокращён до хекс-дампа.
- Удалено то, что не входит в эту сборку: корпус для печати (OpenSCAD), платы RP2040, CI и
  документация исходного проекта.

## Состав

| Путь | Назначение |
| --- | --- |
| `src/`, `include/` | Исходники прошивки |
| `crc/` | Библиотека CRC ([gityf/crc](https://github.com/gityf/crc)) |
| `CMakeLists.txt` | Сборка |
| `pico-sdk/`, `FreeRTOS-Kernel/`, `tcli/` | Git submodules: [Pico SDK](https://github.com/raspberrypi/pico-sdk), [FreeRTOS-Kernel](https://github.com/raspberrypi/FreeRTOS-Kernel), [tcli](https://github.com/dpse/tcli) |

## Сборка

Из каталога `hdmi-cec`: `python3 cec.py setup`, затем `.venv/bin/python cec.py build`.
Параметры платы и пина задаёт [config.cmake](../config.cmake). Запись на плату описана в
[docs/FLASHING.md](../docs/FLASHING.md).
