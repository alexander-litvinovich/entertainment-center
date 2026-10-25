//go:build !windows

package main

import (
	"errors"
	"strings"
)

func platformCommand(args []string) error {
	switch strings.ToLower(args[0]) {
	case "service", "install", "uninstall":
		return errors.New(args[0] + " is available on Windows only; use `hdmi-cec run --dry-run` to try the host here")
	}
	return usage()
}
