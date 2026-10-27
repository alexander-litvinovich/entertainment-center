package ipc

import (
	"context"
	"errors"

	"hdmi-cec-host/internal/core"
)

// Handler executes CLI requests against the controller.
type Handler struct {
	Controller *core.Controller
	// Update installs a newer release, or the latest one with force; nil when the host cannot update itself.
	Update func(ctx context.Context, force bool) (string, error)
}

func (h Handler) Handle(ctx context.Context, req Request) Response {
	var err error
	var message string
	switch req.Command {
	case "update":
		if h.Update == nil {
			err = errors.New("update runs only in the installed service")
		} else {
			message, err = h.Update(ctx, req.Force)
		}
	case "on":
		err = h.Controller.ManualOn(ctx)
	case "off":
		err = h.Controller.Off(ctx)
	case "input":
		err = h.Controller.SetInput(ctx, req.Input)
	case "probe":
		err = h.Controller.Probe(ctx, req.Input)
	case "send":
		err = h.Controller.SendFrame(ctx, req.Input)
	case "config":
		cfg := h.Controller.Config()
		if req.Config != nil {
			cfg, err = h.Controller.SetConfig(*req.Config)
		}
		if err == nil {
			return Response{OK: true, Config: &cfg}
		}
	case "status":
	default:
		err = errors.New("unknown command")
	}
	response := Response{OK: err == nil, Message: message}
	if err != nil {
		response.Message = err.Error()
	}
	status := h.Controller.Status()
	response.Status = &status
	return response
}
