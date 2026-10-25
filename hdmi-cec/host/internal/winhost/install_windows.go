//go:build windows

package winhost

import (
	"fmt"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"os"
	"os/exec"
)

func Install() error {
	_ = exec.Command("schtasks.exe", "/Delete", "/TN", "HDMI CEC Wake", "/F").Run()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if existing, openErr := m.OpenService(ServiceName); openErr == nil {
		existing.Close()
		return fmt.Errorf("service %s is already installed", ServiceName)
	}
	s, err := m.CreateService(ServiceName, exe, mgr.Config{DisplayName: DisplayName, Description: "Host-controlled HDMI-CEC adapter", StartType: mgr.StartAutomatic, ServiceStartName: "LocalSystem"}, "service")
	if err != nil {
		return err
	}
	defer s.Close()
	if err := configureRecovery(s); err != nil {
		_ = s.Delete()
		return err
	}
	if err := eventlog.InstallAsEventCreate(ServiceName, eventlog.Info|eventlog.Warning|eventlog.Error); err != nil {
		_ = s.Delete()
		return err
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("installed, but could not start service: %w", err)
	}
	return nil
}

func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(ServiceName)
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.Delete(); err != nil {
		return err
	}
	_ = eventlog.Remove(ServiceName)
	return nil
}
