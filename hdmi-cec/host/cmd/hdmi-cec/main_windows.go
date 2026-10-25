//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/winhost"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hdmi-cec:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch strings.ToLower(args[0]) {
	case "service":
		programData := os.Getenv("ProgramData")
		if programData == "" {
			return fmt.Errorf("ProgramData is not set")
		}
		return winhost.RunService(winhost.DefaultConfigPath(programData))
	case "install":
		return winhost.Install()
	case "uninstall":
		return winhost.Uninstall()
	case "on", "off", "status":
		if len(args) != 1 {
			return usage()
		}
		return send(ipc.Request{Command: strings.ToLower(args[0])})
	case "input":
		if len(args) != 2 {
			return usage()
		}
		return send(ipc.Request{Command: "input", Input: args[1]})
	default:
		return usage()
	}
}
func send(request ipc.Request) error {
	response, err := winhost.Send(request)
	if err != nil {
		return fmt.Errorf("service is unavailable: %w", err)
	}
	if !response.OK {
		return fmt.Errorf("%s", response.Message)
	}
	if response.Status != nil {
		data, _ := json.MarshalIndent(response.Status, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}
func usage() error {
	return fmt.Errorf("usage: hdmi-cec <on|off|input HDMI1..HDMI4|input 2.1.0.0|status|install|uninstall|service>")
}
