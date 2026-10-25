# Wiring the standalone HDMI-CEC adapter

Board: Seeed Studio XIAO RP2350. One HDMI Type A male plug goes into a free TV port.
The PC's video runs over a separate cable.

| Signal | HDMI pin | XIAO | GPIO |
| --- | --- | --- | --- |
| CEC | 13 | D10 | 3 |
| DDC/CEC Ground | 17 | GND | — |

The XIAO is powered through its USB-C port. HDMI pin 18, pins 15/16 (DDC) and the video lines
stay unconnected. Never join USB 5 V to HDMI 5 V. Check the connector pin numbering from the
solder side: it is mirrored relative to the contact side.

The firmware pulls CEC to ground or releases the line; it never drives it high. The GPIO's
internal pulls are disabled, as upstream; this build relies on the pull-up of the TV's
existing CEC bus. Before the first test check for shorts and measure the idle line level
against HDMI pin 17. Behaviour with the XIAO unpowered, including the effect on the shared CEC
bus and the RP2350 revision, must be checked on the actual build. If the line does not rise or
the adapter disturbs other devices, the hardware interface needs rework; software retries
will not fix it.

The standalone adapter does not touch the video cable and does not announce itself as the
active HDMI source. Without EDID the mode is limited to power-on and basic CEC responses.
Compatibility with every TV is not confirmed.

## Actual build

| Parameter | Value |
| --- | --- |
| Board | Seeed Studio XIAO RP2350 |
| Board / chip revision | Not recorded yet |
| TV model and CEC settings | Not recorded yet |
| Adapter's HDMI port | Not recorded yet |
| USB power source | To be confirmed during testing |
| USB really powers off between boots | To be checked |
| Soldering and idle line level | To be checked |

TV USB power may appear only after the screen turns on, which makes it unsuitable for waking
the TV. Test the PC's USB or a separate power supply first.

## Sources

- [pico-cec: hardware](https://github.com/gkoh/pico-cec#hardware).
- [Seeed: XIAO RP2350](https://wiki.seeedstudio.com/xiao_rp2350_arduino/).
- [RP2350 datasheet](https://datasheets.raspberrypi.com/rp2350/rp2350-datasheet.pdf).
