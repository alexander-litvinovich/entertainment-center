package core

import (
	"context"
	"testing"
)

func TestNextSetupInput(t *testing.T) {
	current, steps := -1, []int{}
	wraps := 0
	for i := 0; i < 6; i++ {
		next, wrapped := NextSetupInput(current)
		if wrapped {
			wraps++
		}
		current = next
		steps = append(steps, current)
	}
	if want := []int{0, 1, 2, 3, 0, 1}; len(steps) != len(want) || steps[4] != 0 || steps[5] != 1 || wraps != 1 {
		t.Fatalf("steps=%v wraps=%d", steps, wraps)
	}
}

func TestProbeDoesNotSave(t *testing.T) {
	d, store := &fakeDevice{}, &memoryStore{cfg: Config{Input: "1.0.0.0", InputDelayMS: 5000}}
	c, _ := NewController(d, store, nil)
	if err := c.Probe(context.Background(), "HDMI3"); err != nil {
		t.Fatal(err)
	}
	if store.cfg.Input != "1.0.0.0" || len(d.commands) != 1 || d.commands[0].Address != 0x3000 {
		t.Fatalf("config=%+v commands=%+v", store.cfg, d.commands)
	}
}
