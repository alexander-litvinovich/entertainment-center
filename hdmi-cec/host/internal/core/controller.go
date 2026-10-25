package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Command struct {
	Name    string
	Address uint16
}

type DeviceStatus struct {
	Connected bool   `json:"connected"`
	Port      string `json:"port,omitempty"`
	Firmware  string `json:"firmware,omitempty"`
}

type Device interface {
	Execute(context.Context, Command) (string, error)
	Status() DeviceStatus
}

type Logger interface {
	Info(string)
	Error(string)
}

type NopLogger struct{}

func (NopLogger) Info(string)  {}
func (NopLogger) Error(string) {}

type Status struct {
	Device     DeviceStatus `json:"device"`
	Input      string       `json:"input,omitempty"`
	DelayMS    int          `json:"inputDelayMs"`
	AutoActive bool         `json:"autoActive"`
	LastAction string       `json:"lastAction,omitempty"`
	LastResult string       `json:"lastResult,omitempty"`
	LastError  string       `json:"lastError,omitempty"`
}

type Controller struct {
	device Device
	store  ConfigStore
	log    Logger
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error

	commandMu  sync.Mutex
	mu         sync.Mutex
	cfg        Config
	autoCancel context.CancelFunc
	autoActive bool
	lastAuto   time.Time
	lastAction string
	lastResult string
	lastError  string
}

func NewController(device Device, store ConfigStore, log Logger) (*Controller, error) {
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = NopLogger{}
	}
	return &Controller{
		device: device, store: store, log: log, cfg: cfg, now: time.Now,
		sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}, nil
}

func (c *Controller) TriggerAuto(parent context.Context, reason string) bool {
	c.mu.Lock()
	now := c.now()
	if c.autoActive || (!c.lastAuto.IsZero() && now.Sub(c.lastAuto) < 30*time.Second) {
		c.mu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(parent)
	c.autoCancel, c.autoActive, c.lastAuto = cancel, true, now
	c.mu.Unlock()
	c.log.Info("automatic CEC sequence: " + reason)
	go func() {
		defer func() {
			c.mu.Lock()
			c.autoActive, c.autoCancel = false, nil
			c.mu.Unlock()
		}()
		_ = c.sequence(ctx)
	}()
	return true
}

func (c *Controller) ManualOn(ctx context.Context) error {
	c.cancelAuto()
	c.log.Info("user action: on")
	return c.sequence(ctx)
}

func (c *Controller) Off(ctx context.Context) error {
	c.cancelAuto()
	c.log.Info("user action: off")
	return c.execute(ctx, Command{Name: "off"})
}

func (c *Controller) SetInput(ctx context.Context, value string) error {
	address, normalized, err := ParseAddress(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	cfg := c.cfg
	cfg.Input = normalized
	c.mu.Unlock()
	if err := c.store.Save(cfg); err != nil {
		return err
	}
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
	c.log.Info("user action: input " + normalized)
	return c.execute(ctx, Command{Name: "input", Address: address})
}

func (c *Controller) sequence(ctx context.Context) error {
	onErr := c.execute(ctx, Command{Name: "on"})
	c.mu.Lock()
	delay := time.Duration(c.cfg.InputDelayMS) * time.Millisecond
	c.mu.Unlock()
	if err := c.sleep(ctx, delay); err != nil {
		if errors.Is(err, context.Canceled) {
			return err
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	input := c.cfg.Input
	c.mu.Unlock()
	if input == "" {
		return onErr
	}
	address, _, err := ParseAddress(input)
	if err != nil {
		return err
	}
	inputErr := c.execute(ctx, Command{Name: "input", Address: address})
	if inputErr != nil {
		return inputErr
	}
	return onErr
}

func (c *Controller) execute(ctx context.Context, command Command) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := c.device.Execute(ctx, command)
	c.mu.Lock()
	c.lastAction, c.lastResult, c.lastError = command.Name, result, ""
	if err != nil {
		c.lastError = err.Error()
	}
	c.mu.Unlock()
	if err != nil {
		c.log.Error(command.Name + ": " + err.Error())
	}
	return err
}

func (c *Controller) cancelAuto() {
	c.mu.Lock()
	if c.autoCancel != nil {
		c.autoCancel()
	}
	c.mu.Unlock()
}

func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{Device: c.device.Status(), Input: c.cfg.Input, DelayMS: c.cfg.InputDelayMS,
		AutoActive: c.autoActive, LastAction: c.lastAction, LastResult: c.lastResult, LastError: c.lastError}
}
