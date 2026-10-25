//go:build windows

package winhost

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/sys/windows"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
	"hdmi-cec-host/internal/ipc"
)

type consoleLogger struct{}

func (consoleLogger) Info(message string)  { log.Println("INFO ", message) }
func (consoleLogger) Error(message string) { log.Println("ERROR", message) }

// RunConsole hosts the controller in the foreground without SCM or Event Log.
// Other terminals reach it through the same named pipe as the service.
func RunConsole(ctx context.Context, configPath string, auto bool) error {
	controller, err := core.NewController(device.New(), core.FileConfigStore{Path: configPath}, consoleLogger{})
	if err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	// Only the current user: a non-elevated process cannot create the service's pipe ACL.
	server, err := StartPipeServer(ctx, controller, "D:P(A;;GA;;;"+user.User.Sid.String()+")", nil)
	if err != nil {
		return fmt.Errorf("start named pipe (is the %s service running? stop it first): %w", ServiceName, err)
	}
	defer server.Close()
	log.Printf("console host running; config %s; pipe %s; Ctrl+C to stop", configPath, ipc.PipeName)
	if auto {
		controller.TriggerAuto(ctx, "console --auto")
	}
	<-ctx.Done()
	return nil
}
