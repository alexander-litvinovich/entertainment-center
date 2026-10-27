//go:build windows

package winhost

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows/svc/mgr"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/update"
)

// Version is the host release version, injected by main from -ldflags.
var Version = "dev"

const (
	updateFirstCheck = 2 * time.Minute
	updateInterval   = 24 * time.Hour
	// exitUpdated makes SCM treat the stop as a failure, so its recovery action restarts the new binary.
	exitUpdated = 10
)

type autoUpdater struct {
	configPath string
	log        core.Logger
	restart    chan struct{}
	mu         sync.Mutex
}

func newAutoUpdater(configPath string, log core.Logger) *autoUpdater {
	return &autoUpdater{configPath: configPath, log: log, restart: make(chan struct{}, 1)}
}

// Run checks GitHub once and installs a newer release, or the latest one with force;
// the service restarts afterwards.
func (a *autoUpdater) Run(ctx context.Context, force bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	updater := update.New(Version)
	updater.Force = force
	release, updated, err := updater.Update(ctx, exe)
	if err != nil {
		return "", err
	}
	if !updated {
		return fmt.Sprintf("version %s is up to date (latest release %s)", Version, release.Version), nil
	}
	select {
	case a.restart <- struct{}{}:
	default:
	}
	return fmt.Sprintf("updated %s -> %s; the service restarts", Version, release.Version), nil
}

func (a *autoUpdater) loop(ctx context.Context) {
	timer := time.NewTimer(updateFirstCheck)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		timer.Reset(updateInterval)
		cfg, err := core.FileConfigStore{Path: a.configPath}.Load()
		if err == nil && !cfg.AutoUpdateEnabled() {
			continue
		}
		message, err := a.Run(ctx, false)
		if err != nil {
			a.log.Error("auto-update: " + err.Error())
			continue
		}
		a.log.Info("auto-update: " + message)
	}
}

func configureRecovery(s *mgr.Service) error {
	restart := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 5 * time.Second}
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{restart, restart, restart}, 24*60*60); err != nil {
		return err
	}
	return s.SetRecoveryActionsOnNonCrashFailures(true)
}

// ensureRecovery upgrades services installed before auto-update existed.
func ensureRecovery() error {
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
	return configureRecovery(s)
}
