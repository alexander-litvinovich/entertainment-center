# Установка Windows Service

## Требования

- Windows 10/11 x64.
- Прошивка адаптера `cec-host-0.6.0` или новее.
- USB CDC с VID:PID `CAFE:4005`.
- Права администратора только для установки и удаления службы.

## Установка

1. Скопируйте `build/windows-amd64/hdmi-cec.exe` в постоянный каталог, например
   `C:\Program Files\HdmiCecHost\`. Не перемещайте EXE после установки.
2. Откройте PowerShell от имени администратора в этом каталоге.
3. Выполните `.\hdmi-cec.exe install`.

Команда удаляет старую задачу `HDMI CEC Wake`, создаёт автоматическую службу
`HdmiCecHost` под `LocalSystem`, регистрирует источник Event Log и запускает службу.
Этот первый запуск имеет причину `Demand`, поэтому сам по себе ТВ не включает.

## Выбор входа и управление

```powershell
.\hdmi-cec.exe input HDMI2
.\hdmi-cec.exe input 2.1.0.0
.\hdmi-cec.exe on
.\hdmi-cec.exe off
.\hdmi-cec.exe status
```

`HDMI1`…`HDMI4` соответствуют `1.0.0.0`…`4.0.0.0`. В raw-адресе четыре hex-ниббла;
после первого нуля разрешены только нули. `input` атомарно сохраняет выбор через службу,
затем переключает ТВ. `on` включает ТВ и через 5 секунд применяет сохранённый вход.
`off` отменяет отложенное переключение и отправляет Standby.

CLI общается только с локальной службой через `\\.\pipe\hdmi-cec-host`. Удалённые
клиенты отклоняются, локальным авторизованным пользователям доступны команды.

## Автоматические события

- `Auto` и `DelayedAuto` при загрузке Windows: `on`, затем input через 5 секунд.
- `PBT_APMRESUMEAUTOMATIC (0x12)`: та же последовательность.
- `PBT_APMRESUMESUSPEND`, ручной start и restart-on-failure: без CEC-команд.
- Повторные boot/resume события за 30 секунд объединяются.

Служба ждёт адаптер до 60 секунд. Прошивка младше `0.6.0` отклоняется. CEC ACK
подтверждает доставку кадра, но не фактическое состояние экрана.

## Диагностика

```powershell
.\hdmi-cec.exe status
Get-WinEvent -LogName Application | Where-Object ProviderName -eq HdmiCecHost | Select-Object -First 20
Get-Service HdmiCecHost
```

Если адаптер не найден, проверьте «Диспетчер устройств», USB-кабель с данными и отсутствие
другой программы, открывшей COM-порт.

Удаление из повышенного PowerShell: `.\hdmi-cec.exe uninstall`.
