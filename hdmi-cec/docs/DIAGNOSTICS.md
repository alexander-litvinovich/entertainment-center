# USB console and CEC diagnostics

The adapter exposes a USB CDC console (VID:PID `CAFE:4005`, 115200 baud). From the repository
root:

```sh
python3 .agents/skills/_shared/cec.py monitor --command "show version"            # one command, prints the reply
python3 .agents/skills/_shared/cec.py monitor --command "cec line" --seconds 1
python3 .agents/skills/_shared/cec.py monitor                                     # interactive, Ctrl+] exits
```

Add `--port /dev/cu.usbmodem…` when several adapters are connected. Only one program can hold
the port: stop the Windows service or close other terminals first.

## Commands

Read-only:

| Command | Output |
| --- | --- |
| `show version` | `cec-host-<version>-<source hash>` |
| `cec status <id>` | State of request `<id>`, see below |
| `cec line` | `@cec line high=N/2000 edges=E`: CEC line sampled for 200 ms |
| `cec scan` | `@cec scan gpioN high=N/200` for every header pin, with pulls disabled |
| `show stats cec`, `show stats tasks`, `show stats cpu` | Counters and FreeRTOS task info |
| `debug on` / `debug off` | Hex dump of every transmitted frame on the console |

These change the CEC bus or the board:

| Command | Effect |
| --- | --- |
| `cec submit <id> raw <hex>` | Transmits one frame (1–16 bytes, first byte `4X`: sender is logical address 4) |
| `cec pullup on` / `off` | Internal pull-up on the CEC pin until reset |
| `reboot` / `reboot bootsel` | Restart, or enter the USB bootloader |

`send`, `set`, `save` and `query` are left over from upstream pico-cec and are not part of the
host protocol.

## Request results

```text
cec submit 1 raw 4004      -> @cec id=1 state=queued
cec status 1               -> @cec id=1 state=done result=ack
```

States go `queued` → `pending` → `done`. Request IDs are decimal numbers ≥ 1. One command runs at a time; a second `submit` while one
is pending returns `state=rejected result=busy`. A malformed frame returns
`state=rejected result=error`, as does `cec status` for an unknown ID.

| Result | Meaning |
| --- | --- |
| `ack` | The frame was acknowledged on the bus |
| `nack` | Nobody acknowledged: the device is absent, CEC is off in the TV menu, or the line is not connected |
| `busy` | The line never became free before transmission |
| `collision` | Arbitration lost: the line read low while the adapter sent `1` |
| `timeout` | The transmission did not finish in time |
| `error` | Internal error |

`ack` confirms delivery, not the screen state.

Typical frames: `4004` Image View On, `4036` Standby, `4f82hhll` Active Source,
`4f84hhll04` Report Physical Address (`hhll` = physical address, e.g. `2000` for HDMI2).

## CEC line

- `cec line` → `high=2000/2000`: the TV pulls the line up; the bus is idle and usable.
- `high=0/2000`: no pull-up: the TV is off or CEC is disabled, the wire is missing, or the
  adapter sits on the wrong pin. Commands then end in `busy` or `collision`.
- A value in between with `edges>0`: there is CEC traffic on the bus.
- `cec scan` shows which header pin actually reads high, which finds a wire on the wrong pin.
  The CEC pin is GPIO3 (D10).
- `cec pullup on` is an experiment for a bus without a pull-up. A following `nack` means the
  line works but nobody acknowledges frames. Reset the board to return to the default.

Wiring and electrical limits: [HARDWARE.md](HARDWARE.md).
