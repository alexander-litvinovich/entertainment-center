package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Command is one CEC frame the adapter sends; Name labels it for status and logs.
type Command struct {
	Name  string
	Frame []byte
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
	c.log.Info("user action: off")
	return c.standby(ctx)
}

// PowerEvent is a Windows power transition the service reacts to.
type PowerEvent string

const (
	Startup  PowerEvent = "startup"
	Wake     PowerEvent = "wake"
	Sleep    PowerEvent = "sleep"
	Shutdown PowerEvent = "shutdown"
)

// HandlePowerEvent turns the TV on or off when the config enables it for the event.
// Turning on runs in the background; turning off returns once Standby is sent.
func (c *Controller) HandlePowerEvent(ctx context.Context, event PowerEvent, detail string) error {
	c.mu.Lock()
	cfg := c.cfg
	c.mu.Unlock()
	var enabled, turnOn bool
	switch event {
	case Startup:
		enabled, turnOn = cfg.TurnOn.Startup, true
	case Wake:
		enabled, turnOn = cfg.TurnOn.Wake, true
	case Sleep:
		enabled = cfg.TurnOff.Sleep
	case Shutdown:
		enabled = cfg.TurnOff.Shutdown
	default:
		return fmt.Errorf("unknown power event %q", event)
	}
	reason := string(event)
	if detail != "" {
		reason += " (" + detail + ")"
	}
	switch {
	case !enabled:
		c.log.Info(reason + ": TV control is disabled for this event")
		return nil
	case turnOn:
		c.TriggerAuto(ctx, reason)
		return nil
	default:
		c.log.Info("automatic standby: " + reason)
		return c.standby(ctx)
	}
}

// Config returns the active configuration.
func (c *Controller) Config() Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

// SetConfig validates and saves a whole configuration without sending CEC frames.
func (c *Controller) SetConfig(cfg Config) (Config, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return cfg, err
	}
	if err := c.store.Save(cfg); err != nil {
		return cfg, err
	}
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
	c.log.Info(fmt.Sprintf("config saved: input %q, turn on %+v, turn off %+v", cfg.Input, cfg.TurnOn, cfg.TurnOff))
	return cfg, nil
}

// standby cancels a pending input switch, announces the adapter and sends Standby.
func (c *Controller) standby(ctx context.Context) error {
	c.cancelAuto()
	// The 30 s coalescing only drops duplicate turn-on events; after Standby the next one must run.
	c.mu.Lock()
	c.lastAuto = time.Time{}
	c.mu.Unlock()
	announceErr := c.announce(ctx)
	if err := c.execute(ctx, Command{Name: "off", Frame: FrameOff()}); err != nil {
		return err
	}
	return announceErr
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
	return c.switchInput(ctx, address)
}

// Probe switches the TV to an input without saving it.
func (c *Controller) Probe(ctx context.Context, value string) error {
	address, normalized, err := ParseAddress(value)
	if err != nil {
		return err
	}
	c.cancelAuto()
	c.log.Info("user action: probe input " + normalized)
	return c.switchInput(ctx, address)
}

// SendFrame sends one raw CEC frame such as "4F:82:20:00" for experiments.
func (c *Controller) SendFrame(ctx context.Context, text string) error {
	frame, err := ParseFrame(text)
	if err != nil {
		return err
	}
	c.cancelAuto()
	c.log.Info("user action: send " + FormatFrame(frame))
	return c.execute(ctx, Command{Name: "raw", Frame: frame})
}

// switchInput registers the adapter at the input's physical address, then announces it as active.
func (c *Controller) switchInput(ctx context.Context, address uint16) error {
	reportErr := c.execute(ctx, Command{Name: "report", Frame: FrameReportPhysicalAddress(address)})
	if err := c.execute(ctx, Command{Name: "active", Frame: FrameActiveSource(address)}); err != nil {
		return err
	}
	return reportErr
}

// announce reports the saved input's physical address so the TV knows this device; no-op without one.
func (c *Controller) announce(ctx context.Context) error {
	c.mu.Lock()
	input := c.cfg.Input
	c.mu.Unlock()
	if input == "" {
		return nil
	}
	address, _, err := ParseAddress(input)
	if err != nil {
		return err
	}
	return c.execute(ctx, Command{Name: "report", Frame: FrameReportPhysicalAddress(address)})
}

func (c *Controller) sequence(ctx context.Context) error {
	onErr := c.execute(ctx, Command{Name: "on", Frame: FrameOn()})
	c.mu.Lock()
	delay := time.Duration(c.cfg.InputDelayMS) * time.Millisecond
	c.mu.Unlock()
	if err := c.sleep(ctx, delay); err != nil {
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
	if err := c.switchInput(ctx, address); err != nil {
		return err
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
