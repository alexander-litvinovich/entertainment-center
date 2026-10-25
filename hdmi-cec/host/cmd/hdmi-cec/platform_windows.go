//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"

	"hdmi-cec-host/internal/winhost"
)

func init() { winhost.Version = version }

func platformCommand(args []string) error {
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
	}
	return usage()
}
