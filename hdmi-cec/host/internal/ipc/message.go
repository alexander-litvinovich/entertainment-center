package ipc

import "hdmi-cec-host/internal/core"

type Request struct {
	Command string       `json:"command"`
	Input   string       `json:"input,omitempty"`
	Force   bool         `json:"force,omitempty"`
	Config  *core.Config `json:"config,omitempty"`
}

type Response struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message,omitempty"`
	Status  *core.Status `json:"status,omitempty"`
	Config  *core.Config `json:"config,omitempty"`
}

// Access chooses who may connect to the host.
type Access int

const (
	// CurrentUser admits only the user running the host (console and dry-run hosts).
	CurrentUser Access = iota
	// LocalUsers admits SYSTEM, administrators and authenticated local users (the service).
	LocalUsers
)
