package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingConfigUsesDefaults(t *testing.T) {
	cfg, err := FileConfigStore{Path: filepath.Join(t.TempDir(), "config.json")}.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadLegacyConfigKeepsTVOnAfterSleep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"input":"hdmi2","inputDelayMs":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := FileConfigStore{Path: path}.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Input: "2.0.0.0", InputDelayMS: DefaultInputDelayMS, TurnOn: TurnOnEvents{Wake: true, Startup: true}}
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	store := FileConfigStore{Path: filepath.Join(t.TempDir(), "sub", "config.json")}
	saved := Config{Input: "3.1.0.0", InputDelayMS: 2000, TurnOn: TurnOnEvents{Startup: true}, TurnOff: TurnOffEvents{Shutdown: true}}
	if err := store.Save(saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != saved {
		t.Fatalf("loaded %+v, saved %+v", loaded, saved)
	}
}
