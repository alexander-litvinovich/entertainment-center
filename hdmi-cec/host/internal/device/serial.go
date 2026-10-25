package device

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"

	"hdmi-cec-host/internal/core"
)

const (
	wantedVID = "CAFE"
	wantedPID = "4005"
)

var responseRE = regexp.MustCompile(`@cec id=([0-9]+) state=(queued|pending|done|rejected)(?: result=(ack|nack|busy|timeout|collision|error))?`)
var versionRE = regexp.MustCompile(`cec-host-([0-9]+)\.([0-9]+)\.([0-9]+)(?:-[0-9a-f]+)?`)

type SerialDevice struct {
	mu       sync.Mutex // serializes port I/O
	port     serial.Port
	portName string
	firmware string
	buffer   string
	nextID   atomic.Uint32
	wait     time.Duration

	stateMu sync.Mutex // guards status so Status never waits for the adapter
	status  core.DeviceStatus
}

func New() *SerialDevice { return &SerialDevice{wait: 60 * time.Second} }

func (d *SerialDevice) Status() core.DeviceStatus {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.status
}

func (d *SerialDevice) setStatus(s core.DeviceStatus) {
	d.stateMu.Lock()
	d.status = s
	d.stateMu.Unlock()
}

func (d *SerialDevice) Execute(ctx context.Context, command core.Command) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpen(ctx); err != nil {
		return "error", err
	}
	id := d.nextID.Add(1)
	if id == 0 {
		id = d.nextID.Add(1)
	}
	line := fmt.Sprintf("cec submit %d raw %s", id, hex.EncodeToString(command.Frame))
	err := d.writeLine(line)
	if err != nil {
		// The handle goes stale when the adapter re-enumerates, e.g. after sleep.
		d.close()
		if err = d.ensureOpen(ctx); err == nil {
			err = d.writeLine(line)
		}
	}
	if err != nil {
		d.close()
		return "error", err
	}
	deadline := time.Now().Add(12 * time.Second)
	for {
		state, result, found, err := d.readResponse(ctx, id, deadline)
		if err != nil {
			d.close()
			return "error", err
		}
		if found && state == "done" {
			return result, nil
		}
		if found && state == "rejected" {
			return result, fmt.Errorf("firmware rejected command: %s", result)
		}
		if time.Now().After(deadline) {
			return "timeout", errors.New("firmware command timed out")
		}
		if err := d.writeLine(fmt.Sprintf("cec status %d", id)); err != nil {
			d.close()
			return "error", err
		}
	}
}

func (d *SerialDevice) ensureOpen(ctx context.Context) error {
	if d.port != nil {
		return nil
	}
	deadline := time.Now().Add(d.wait)
	for {
		ports, err := enumerator.GetDetailedPortsList()
		if err == nil {
			for _, info := range ports {
				if info.IsUSB && strings.EqualFold(info.VID, wantedVID) && strings.EqualFold(info.PID, wantedPID) {
					p, openErr := serial.Open(info.Name, &serial.Mode{BaudRate: 115200})
					if openErr != nil {
						err = openErr
						continue
					}
					_ = p.SetReadTimeout(100 * time.Millisecond)
					d.port, d.portName = p, info.Name
					if versionErr := d.checkVersion(ctx); versionErr != nil {
						d.close()
						return versionErr
					}
					d.setStatus(core.DeviceStatus{Connected: true, Port: d.portName, Firmware: d.firmware})
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("find USB CEC adapter: %w", err)
			}
			return errors.New("USB CEC adapter CAFE:4005 not found within 60 seconds")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (d *SerialDevice) checkVersion(ctx context.Context) error {
	if err := d.writeLine("show version"); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	buffer := ""
	chunk := make([]byte, 256)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := d.port.Read(chunk)
		if err != nil {
			return err
		}
		if n > 0 {
			buffer += string(chunk[:n])
			match := versionRE.FindStringSubmatch(buffer)
			if match != nil {
				major, _ := strconv.Atoi(match[1])
				minor, _ := strconv.Atoi(match[2])
				if major != 0 || minor < 7 {
					return fmt.Errorf("firmware protocol %s is too old; version 0.7.0 or newer is required", match[0])
				}
				d.firmware = match[0]
				return nil
			}
		}
	}
	return errors.New("firmware did not report a compatible cec-host version")
}

func (d *SerialDevice) writeLine(line string) error {
	_, err := d.port.Write([]byte("\r" + line + "\r"))
	return err
}

func (d *SerialDevice) readResponse(ctx context.Context, id uint32, deadline time.Time) (string, string, bool, error) {
	chunk := make([]byte, 256)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", "", false, ctx.Err()
		default:
		}
		n, err := d.port.Read(chunk)
		if err != nil {
			return "", "", false, err
		}
		if n == 0 {
			return "", "", false, nil
		}
		d.buffer += string(chunk[:n])
		if len(d.buffer) > 4096 {
			d.buffer = d.buffer[len(d.buffer)-4096:]
		}
		matches := responseRE.FindAllStringSubmatch(d.buffer, -1)
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			responseID, _ := strconv.ParseUint(match[1], 10, 32)
			if uint32(responseID) == id {
				return match[2], match[3], true, nil
			}
		}
	}
	return "", "", false, nil
}

func (d *SerialDevice) close() {
	if d.port != nil {
		_ = d.port.Close()
	}
	d.port, d.portName, d.firmware, d.buffer = nil, "", "", ""
	d.setStatus(core.DeviceStatus{})
}

func ParseResponse(text string, id uint32) (state, result string, ok bool) {
	for _, match := range responseRE.FindAllStringSubmatch(text, -1) {
		responseID, _ := strconv.ParseUint(match[1], 10, 32)
		if uint32(responseID) == id {
			return match[2], match[3], true
		}
	}
	return "", "", false
}
