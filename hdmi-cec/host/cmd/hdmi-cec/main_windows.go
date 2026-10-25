//go:build windows

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/winhost"
	"os"
	"os/signal"
	"path/filepath"
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
	case "run":
		return runConsole(args[1:])
	case "setup":
		if len(args) != 1 {
			return usage()
		}
		return runSetup()
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
func runConsole(args []string) error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	auto := flags.Bool("auto", false, "run the boot/resume sequence (on, delay, saved input) at start")
	config := flags.String("config", filepath.Join(dir, "HdmiCecHost", "config.json"), "config file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usage()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return winhost.RunConsole(ctx, *config, *auto)
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
	return fmt.Errorf("usage: hdmi-cec <on|off|input HDMI1..HDMI4|input 2.1.0.0|status|setup|run [--auto] [--config file]|install|uninstall|service>")
}
