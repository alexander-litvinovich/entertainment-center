package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/tui"
)

// version is set at release build time: -ldflags "-X main.version=0.7.0".
var version = "dev"

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
	case "run":
		return runConsole(args[1:])
	case "setup":
		return runSetup(args[1:])
	case "version":
		fmt.Println(version)
		return nil
	case "on", "off", "status", "update":
		if len(args) != 1 {
			return usage()
		}
		return send(ipc.Request{Command: strings.ToLower(args[0])})
	case "send":
		if len(args) < 2 {
			return usage()
		}
		return send(ipc.Request{Command: "send", Input: strings.Join(args[1:], " ")})
	case "input":
		if len(args) != 2 {
			return usage()
		}
		return send(ipc.Request{Command: "input", Input: args[1]})
	default:
		return platformCommand(args)
	}
}

func defaultConsoleConfig() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "HdmiCecHost", "config.json")
}

type consoleLogger struct{}

func (consoleLogger) Info(message string)  { log.Println("INFO ", message) }
func (consoleLogger) Error(message string) { log.Println("ERROR", message) }

// runConsole hosts the controller in the foreground; other terminals reach it like the service.
func runConsole(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	auto := flags.Bool("auto", false, "run the startup sequence (on, delay, saved input) at start")
	dryRun := flags.Bool("dry-run", false, "log CEC frames instead of sending them; no adapter needed")
	config := flags.String("config", defaultConsoleConfig(), "config file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usage()
	}
	var cec core.Device = device.New()
	if *dryRun {
		cec = device.DryRun{Log: func(message string) { log.Println("CEC  ", message) }}
	}
	controller, err := core.NewController(cec, core.FileConfigStore{Path: *config}, consoleLogger{})
	if err != nil {
		return err
	}
	listener, err := ipc.Listen(ipc.CurrentUser)
	if err != nil {
		return fmt.Errorf("listen on %s (is the service or another `hdmi-cec run` running? stop it first): %w", ipc.Address, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := ipc.Serve(ctx, listener, ipc.Handler{Controller: controller})
	defer server.Close()
	mode := "adapter"
	if *dryRun {
		mode = "dry run"
	}
	log.Printf("console host running (%s); config %s; listening on %s; Ctrl+C to stop", mode, *config, ipc.Address)
	if *auto {
		controller.TriggerAuto(ctx, "console --auto")
	}
	<-ctx.Done()
	return nil
}

// runSetup opens the settings screens against the running host, or in-process with --dry-run.
func runSetup(args []string) error {
	flags := flag.NewFlagSet("setup", flag.ContinueOnError)
	dryRun := flags.Bool("dry-run", false, "edit a local config with a simulated adapter; no service needed")
	config := flags.String("config", defaultConsoleConfig(), "config file for --dry-run")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return usage()
	}
	sendFunc, note := tui.Send(ipc.Send), ""
	if *dryRun {
		controller, err := core.NewController(device.DryRun{}, core.FileConfigStore{Path: *config}, nil)
		if err != nil {
			return err
		}
		handler := ipc.Handler{Controller: controller}
		sendFunc = func(req ipc.Request) (ipc.Response, error) { return handler.Handle(context.Background(), req), nil }
		note = "dry run · " + *config
	} else if _, err := ipc.Send(ipc.Request{Command: "status"}); err != nil {
		return fmt.Errorf("the service is unavailable (install it, or start `hdmi-cec run` in another terminal; `hdmi-cec setup --dry-run` works without it): %w", err)
	}
	_, err := tui.RunSetup(sendFunc, note)
	return err
}

func send(request ipc.Request) error {
	response, err := ipc.Send(request)
	if err != nil {
		return fmt.Errorf("service is unavailable: %w", err)
	}
	if !response.OK {
		return errors.New(response.Message)
	}
	if response.Message != "" {
		fmt.Println(response.Message)
	} else if response.Status != nil {
		data, _ := json.MarshalIndent(response.Status, "", "  ")
		fmt.Println(string(data))
	}
	return nil
}

func usage() error {
	return errors.New(`usage: hdmi-cec <command>

  setup [--dry-run] [--config file]   choose the TV input and when the TV turns on and off
  on | off                            turn the TV on (then select the input) | standby
  input HDMI1..HDMI4 | 2.1.0.0        save the input and switch the TV to it
  send 4F:82:20:00                    send one raw CEC frame
  status | update | version
  run [--auto] [--dry-run] [--config file]
                                      host in this terminal instead of the service
  install | uninstall | service       Windows service (install/uninstall need an elevated shell)`)
}
