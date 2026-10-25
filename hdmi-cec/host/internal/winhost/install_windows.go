//go:build windows

package winhost

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

// Install creates and starts the service. If it already exists, Install points it at this EXE
// and starts it, so re-running it upgrades or moves an installation.
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
		defer existing.Close()
		return reconfigure(existing, exe)
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

func reconfigure(s *mgr.Service, exe string) error {
	cfg, err := s.Config()
	if err != nil {
		return err
	}
	cfg.BinaryPathName = syscall.EscapeArg(exe) + " service"
	cfg.StartType = mgr.StartAutomatic
	if err := s.UpdateConfig(cfg); err != nil {
		return err
	}
	if err := configureRecovery(s); err != nil {
		return err
	}
	status, err := s.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Running {
		fmt.Println("service is already installed and running; stop it first to run a new EXE")
		return nil
	}
	if err := s.Start(); err != nil {
		return fmt.Errorf("updated the service, but could not start it: %w", err)
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
