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
	"hdmi-cec-host/internal/update"
)

const (
	ServiceName           = "HdmiCecHost"
	DisplayName           = "HDMI CEC Host"
	pbtAPMResumeAutomatic = 0x12
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
	server, err := StartPipeServer(ctx, controller, pipeSDDL, updater.Run)
	if err != nil {
		logger.Error("start named pipe: " + err.Error())
		return false, 3
	}
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
		controller.TriggerAuto(ctx, fmt.Sprintf("service start reason %d", reason))
	} else {
		logger.Info(fmt.Sprintf("service start reason %d does not trigger TV", reason))
	}
	accepted := svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent
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
			if request.EventType == pbtAPMResumeAutomatic {
				controller.TriggerAuto(ctx, "resume automatic")
			}
		case svc.Stop, svc.Shutdown:
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
