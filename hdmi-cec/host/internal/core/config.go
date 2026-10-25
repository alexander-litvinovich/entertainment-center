package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const DefaultInputDelayMS = 5000

type Config struct {
	Input        string        `json:"input,omitempty"`
	InputDelayMS int           `json:"inputDelayMs"`
	TurnOn       TurnOnEvents  `json:"turnOn"`
	TurnOff      TurnOffEvents `json:"turnOff"`
	AutoUpdate   *bool         `json:"autoUpdate,omitempty"`
}

// TurnOnEvents selects the Windows events that turn the TV on and select the input.
type TurnOnEvents struct {
	Wake    bool `json:"wake"`
	Startup bool `json:"startup"`
}

// TurnOffEvents selects the Windows events that put the TV into standby.
type TurnOffEvents struct {
	Sleep    bool `json:"sleep"`
	Shutdown bool `json:"shutdown"`
}

// DefaultConfig is the configuration of a fresh install: the TV follows the PC.
func DefaultConfig() Config {
	return Config{
		InputDelayMS: DefaultInputDelayMS,
		TurnOn:       TurnOnEvents{Wake: true, Startup: true},
		TurnOff:      TurnOffEvents{Sleep: true, Shutdown: true},
	}
}

// AutoUpdateEnabled defaults to true when the setting is absent.
func (c Config) AutoUpdateEnabled() bool { return c.AutoUpdate == nil || *c.AutoUpdate }

type ConfigStore interface {
	Load() (Config, error)
	Save(Config) error
}

type FileConfigStore struct{ Path string }

func (s FileConfigStore) Load() (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	// Configs written before turnOff existed never turned the TV off; keep that.
	cfg.TurnOff = TurnOffEvents{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	cfg, err = cfg.Normalize()
	if err != nil {
		return cfg, fmt.Errorf("stored input: %w", err)
	}
	return cfg, nil
}

// Normalize validates the input, rewrites it as a physical address and fills the default delay.
func (c Config) Normalize() (Config, error) {
	if c.InputDelayMS <= 0 {
		c.InputDelayMS = DefaultInputDelayMS
	}
	if c.Input != "" {
		_, normalized, err := ParseAddress(c.Input)
		if err != nil {
			return c, err
		}
		c.Input = normalized
	}
	return c, nil
}

func (s FileConfigStore) Save(cfg Config) error {
	if cfg.InputDelayMS <= 0 {
		cfg.InputDelayMS = DefaultInputDelayMS
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(tmpName, s.Path)
}
