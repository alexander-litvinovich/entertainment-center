package device

import (
	"context"

	"hdmi-cec-host/internal/core"
)

// DryRun stands in for the adapter on machines without one: it reports each frame through
// Log and acknowledges it.
type DryRun struct{ Log func(string) }

func (d DryRun) Execute(_ context.Context, command core.Command) (string, error) {
	if d.Log != nil {
		d.Log("dry run: " + command.Name + " " + core.FormatFrame(command.Frame))
	}
	return "ack", nil
}

func (DryRun) Status() core.DeviceStatus {
	return core.DeviceStatus{Connected: true, Port: "dry-run", Firmware: "dry-run"}
}
