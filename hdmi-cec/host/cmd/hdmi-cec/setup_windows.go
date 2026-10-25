//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/winhost"
)

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
	raw := mode &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT)
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
	key := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(key); err != nil {
			return err
		}
		switch key[0] {
		case ' ':
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
		case '\r', '\n':
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
		case 0x1b, 0x03:
			fmt.Println("Cancelled; config unchanged.")
			return nil
		}
	}
}
