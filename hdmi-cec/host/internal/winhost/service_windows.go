//go:build windows

package winhost

import (
	"context"
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
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
	server, err := StartPipeServer(ctx, controller, pipeSDDL)
	if err != nil {
		logger.Error("start named pipe: " + err.Error())
		return false, 3
	}
	defer server.Close()
	logger.Info("service started")
	if reason, reasonErr := svc.DynamicStartReason(); reasonErr != nil {
		logger.Error("read service start reason: " + reasonErr.Error())
	} else if reason == svc.StartReasonAuto || reason == svc.StartReasonDelayedAuto {
		controller.TriggerAuto(ctx, fmt.Sprintf("service start reason %d", reason))
	} else {
		logger.Info(fmt.Sprintf("service start reason %d does not trigger TV", reason))
	}
	accepted := svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPowerEvent
	statuses <- svc.Status{State: svc.Running, Accepts: accepted}
	for request := range requests {
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
	return false, 0
}
func DefaultConfigPath(programData string) string {
	return filepath.Join(programData, "HdmiCecHost", "config.json")
}
