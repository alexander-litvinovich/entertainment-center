//go:build windows

package winhost

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
	"hdmi-cec-host/internal/ipc"
	"hdmi-cec-host/internal/update"
)

const (
	ServiceName           = "HdmiCecHost"
	DisplayName           = "HDMI CEC Host"
	pbtAPMSuspend         = 0x4
	pbtAPMResumeAutomatic = 0x12
	// Windows does not wait for a service to handle suspend, and waits only seconds at shutdown.
	standbyTimeout = 5 * time.Second
)

type eventLogger struct{ log *eventlog.Log }

func (l eventLogger) Info(message string)  { _ = l.log.Info(1, message) }
func (l eventLogger) Error(message string) { _ = l.log.Error(2, message) }

type serviceHandler struct{ configPath string }

func RunService(configPath string) error {
	return svc.Run(ServiceName, serviceHandler{configPath: configPath})
}

func (h serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	log, err := eventlog.Open(ServiceName)
	if err != nil {
		return false, 1
	}
	defer log.Close()
	logger := eventLogger{log}
	controller, err := core.NewController(device.New(), core.FileConfigStore{Path: h.configPath}, logger)
	if err != nil {
		logger.Error("load config: " + err.Error())
		return false, 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updater := newAutoUpdater(h.configPath, logger)
	listener, err := ipc.Listen(ipc.LocalUsers)
	if err != nil {
		logger.Error("start named pipe: " + err.Error())
		return false, 3
	}
	server := ipc.Serve(ctx, listener, ipc.Handler{Controller: controller, Update: updater.Run})
	defer server.Close()
	logger.Info("service started, version " + Version)
	if exe, exeErr := os.Executable(); exeErr == nil {
		update.Cleanup(exe)
	}
	if err := ensureRecovery(); err != nil {
		logger.Error("configure restart-on-failure: " + err.Error())
	}
	go updater.loop(ctx)
	if reason, reasonErr := svc.DynamicStartReason(); reasonErr != nil {
		logger.Error("read service start reason: " + reasonErr.Error())
	} else if reason == svc.StartReasonAuto || reason == svc.StartReasonDelayedAuto {
		_ = controller.HandlePowerEvent(ctx, core.Startup, fmt.Sprintf("service start reason %d", reason))
	} else {
		logger.Info(fmt.Sprintf("service start reason %d does not trigger TV", reason))
	}
	standby := func(event core.PowerEvent, detail string) {
		standbyCtx, cancelStandby := context.WithTimeout(ctx, standbyTimeout)
		defer cancelStandby()
		_ = controller.HandlePowerEvent(standbyCtx, event, detail)
	}
	// Preshutdown makes SCM wait until the service stops, so Standby goes out before power-off.
	accepted := svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPreShutdown | svc.AcceptPowerEvent
	statuses <- svc.Status{State: svc.Running, Accepts: accepted}
	for {
		var request svc.ChangeRequest
		select {
		case request = <-requests:
		case <-updater.restart:
			// Let a running on -> input sequence finish before the restart.
			for i := 0; i < 30 && controller.Status().AutoActive; i++ {
				time.Sleep(time.Second)
			}
			statuses <- svc.Status{State: svc.StopPending}
			cancel()
			logger.Info("service stopping to restart the updated binary")
			return true, exitUpdated
		}
		switch request.Cmd {
		case svc.Interrogate:
			statuses <- request.CurrentStatus
		case svc.PowerEvent:
			logger.Info(fmt.Sprintf("power event 0x%X", request.EventType))
			switch request.EventType {
			case pbtAPMResumeAutomatic:
				_ = controller.HandlePowerEvent(ctx, core.Wake, "resume automatic")
			case pbtAPMSuspend:
				standby(core.Sleep, "suspend")
			}
		case svc.PreShutdown, svc.Shutdown:
			statuses <- svc.Status{State: svc.StopPending, WaitHint: uint32((standbyTimeout + time.Second).Milliseconds())}
			standby(core.Shutdown, "system shutdown")
			cancel()
			logger.Info("service stopped for system shutdown")
			return false, 0
		case svc.Stop:
			statuses <- svc.Status{State: svc.StopPending}
			cancel()
			logger.Info("service stopped")
			return false, 0
		}
	}
}
func DefaultConfigPath(programData string) string {
	return filepath.Join(programData, "HdmiCecHost", "config.json")
}
