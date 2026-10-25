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
	Input        string `json:"input,omitempty"`
	InputDelayMS int    `json:"inputDelayMs"`
}

type ConfigStore interface {
	Load() (Config, error)
	Save(Config) error
}

type FileConfigStore struct{ Path string }

func (s FileConfigStore) Load() (Config, error) {
	cfg := Config{InputDelayMS: DefaultInputDelayMS}
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if cfg.InputDelayMS <= 0 {
		cfg.InputDelayMS = DefaultInputDelayMS
	}
	if cfg.Input != "" {
		_, normalized, err := ParseAddress(cfg.Input)
		if err != nil {
			return cfg, fmt.Errorf("stored input: %w", err)
		}
		cfg.Input = normalized
	}
	return cfg, nil
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
