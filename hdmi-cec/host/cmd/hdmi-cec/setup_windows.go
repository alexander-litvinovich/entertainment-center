//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/winhost"
)

// inputRecord is a Win32 INPUT_RECORD holding a KEY_EVENT_RECORD.
type inputRecord struct {
	EventType       uint16
	_               uint16
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	Char            uint16
	ControlKeyState uint32
}

const (
	keyEvent         = 1
	ctrlPressedState = 0x0004 | 0x0008 // LEFT_CTRL_PRESSED | RIGHT_CTRL_PRESSED
)

var readConsoleInput = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReadConsoleInputW")

// readKey blocks for the next recognised key press. Console key events keep a lone Esc
// distinguishable, which byte-stream reads do not.
func readKey(in windows.Handle) (core.SetupKey, error) {
	for {
		var record inputRecord
		var read uint32
		ok, _, err := readConsoleInput.Call(uintptr(in), uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&read)))
		if ok == 0 {
			return core.KeyOther, err
		}
		if record.EventType != keyEvent || record.KeyDown == 0 {
			continue
		}
		ctrl := record.ControlKeyState&ctrlPressedState != 0
		if key := core.ClassifySetupKey(record.VirtualKeyCode, record.Char, ctrl); key != core.KeyOther {
			return key, nil
		}
	}
}

func beep() {
	_, _, _ = windows.NewLazySystemDLL("kernel32.dll").NewProc("Beep").Call(880, 200)
}

// runSetup steps the TV through HDMI1..HDMI4 and saves the input the user confirms.
func runSetup() error {
	in := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(in, &mode); err != nil {
		return fmt.Errorf("setup needs an interactive console: %w", err)
	}
	if _, err := winhost.Send(ipc.Request{Command: "status"}); err != nil {
		return fmt.Errorf("service is unavailable (start it, or use `hdmi-cec run`): %w", err)
	}
	// Processed input is off so Ctrl+C reaches us and the console mode is restored.
	raw := mode &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	if err := windows.SetConsoleMode(in, raw); err != nil {
		return err
	}
	defer windows.SetConsoleMode(in, mode)

	fmt.Println("Input setup: the TV will be switched through the HDMI inputs one by one.")
	fmt.Println("Make sure the TV is on.")
	fmt.Println("  Space  switch to the next input (a beep means the list restarts)")
	fmt.Println("  Enter  this is the right input: save it")
	fmt.Println("  Esc    quit without saving")
	fmt.Println()
	fmt.Println("Press Space to switch to the first input.")

	current := -1
	for {
		key, err := readKey(in)
		if err != nil {
			return err
		}
		switch key {
		case core.KeySpace:
			next, wrapped := core.NextSetupInput(current)
			if wrapped {
				beep()
			}
			current = next
			name := core.SetupInputs[current]
			response, err := winhost.Send(ipc.Request{Command: "probe", Input: name})
			switch {
			case err != nil:
				fmt.Printf("%s: service error: %v\n", name, err)
			case !response.OK:
				fmt.Printf("%s: failed: %s\n", name, response.Message)
			default:
				fmt.Printf("%s: sent (CEC result: %s). Space = next, Enter = save, Esc = quit.\n", name, response.Status.LastResult)
			}
		case core.KeyEnter:
			if current < 0 {
				fmt.Println("No input selected yet: press Space first.")
				continue
			}
			name := core.SetupInputs[current]
			response, err := winhost.Send(ipc.Request{Command: "input", Input: name})
			if err != nil {
				return err
			}
			if !response.OK {
				return errors.New(response.Message)
			}
			fmt.Printf("Saved %s (%s) as the input to select after the TV turns on.\n", name, response.Status.Input)
			return nil
		case core.KeyQuit:
			fmt.Println("Cancelled; config unchanged.")
			return nil
		}
	}
}
