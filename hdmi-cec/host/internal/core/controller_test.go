package core

import (
	"context"
	"sync"
	"testing"
	"time"
)

type memoryStore struct{ cfg Config }

func (s *memoryStore) Load() (Config, error) { return s.cfg, nil }
func (s *memoryStore) Save(c Config) error   { s.cfg = c; return nil }

type fakeDevice struct {
	mu       sync.Mutex
	commands []Command
}

func (d *fakeDevice) Execute(_ context.Context, c Command) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.commands = append(d.commands, c)
	return "ack", nil
}
func (d *fakeDevice) Status() DeviceStatus {
	return DeviceStatus{Connected: true, Firmware: "cec-host-0.6.0-test"}
}
func (d *fakeDevice) names() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.commands))
	for i := range d.commands {
		out[i] = d.commands[i].Name
	}
	return out
}

func TestAutomaticSequenceAndDedup(t *testing.T) {
	d := &fakeDevice{}
	c, _ := NewController(d, &memoryStore{Config{Input: "2.0.0.0", InputDelayMS: 1}}, nil)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	if !c.TriggerAuto(context.Background(), "boot") {
		t.Fatal("first event rejected")
	}
	if c.TriggerAuto(context.Background(), "resume") {
		t.Fatal("duplicate accepted")
	}
	deadline := time.Now().Add(time.Second)
	for len(d.names()) < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	got := d.names()
	if len(got) != 2 || got[0] != "on" || got[1] != "input" {
		t.Fatalf("commands = %v", got)
	}
}

func TestOffCancelsDelayedInput(t *testing.T) {
	d := &fakeDevice{}
	c, _ := NewController(d, &memoryStore{Config{Input: "2.0.0.0", InputDelayMS: 5000}}, nil)
	entered := make(chan struct{})
	c.sleep = func(ctx context.Context, _ time.Duration) error { close(entered); <-ctx.Done(); return ctx.Err() }
	c.TriggerAuto(context.Background(), "resume")
	<-entered
	if err := c.Off(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	got := d.names()
	if len(got) != 2 || got[0] != "on" || got[1] != "off" {
		t.Fatalf("commands = %v", got)
	}
}

func TestInputPersistsAndSends(t *testing.T) {
	d, store := &fakeDevice{}, &memoryStore{cfg: Config{InputDelayMS: 5000}}
	c, _ := NewController(d, store, nil)
	if err := c.SetInput(context.Background(), "HDMI3"); err != nil {
		t.Fatal(err)
	}
	if store.cfg.Input != "3.0.0.0" || d.commands[0].Address != 0x3000 {
		t.Fatalf("config=%+v commands=%+v", store.cfg, d.commands)
	}
}
