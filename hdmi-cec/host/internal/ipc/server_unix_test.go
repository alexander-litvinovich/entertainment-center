//go:build !windows

package ipc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
)

func TestServeOverUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "cec")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	Address = filepath.Join(dir, "s.sock")
	store := core.FileConfigStore{Path: filepath.Join(dir, "config.json")}
	controller, err := core.NewController(device.DryRun{}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := Listen(CurrentUser)
	if err != nil {
		t.Fatal(err)
	}
	server := Serve(context.Background(), listener, Handler{Controller: controller})
	defer server.Close()
	if _, err := Listen(CurrentUser); err == nil {
		t.Fatal("second host started on the same socket")
	}

	response, err := Send(Request{Command: "config"})
	if err != nil || !response.OK || response.Config == nil || *response.Config != core.DefaultConfig() {
		t.Fatalf("get config: %+v %v", response, err)
	}
	cfg := *response.Config
	cfg.Input, cfg.TurnOff.Sleep = "HDMI2", false
	response, err = Send(Request{Command: "config", Config: &cfg})
	if err != nil || !response.OK || response.Config.Input != "2.0.0.0" {
		t.Fatalf("set config: %+v %v", response, err)
	}
	if saved, _ := store.Load(); saved.Input != "2.0.0.0" || saved.TurnOff.Sleep {
		t.Fatalf("saved %+v", saved)
	}
	response, err = Send(Request{Command: "probe", Input: "HDMI3"})
	if err != nil || !response.OK || response.Status.LastResult != "ack" || response.Status.Input != "2.0.0.0" {
		t.Fatalf("probe: %+v %v", response, err)
	}
	if response, _ = Send(Request{Command: "update"}); response.OK {
		t.Fatal("update accepted without an updater")
	}
}
