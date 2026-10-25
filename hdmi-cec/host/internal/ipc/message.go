package ipc

import "hdmi-cec-host/internal/core"

const PipeName = `\\.\pipe\hdmi-cec-host`

type Request struct {
	Command string `json:"command"`
	Input   string `json:"input,omitempty"`
}

type Response struct {
	OK      bool         `json:"ok"`
	Message string       `json:"message,omitempty"`
	Status  *core.Status `json:"status,omitempty"`
}
