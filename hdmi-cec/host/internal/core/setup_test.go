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
	if store.cfg.Input != "1.0.0.0" || len(d.commands) != 2 || FormatFrame(d.commands[1].Frame) != "4F:82:30:00" {
		t.Fatalf("config=%+v commands=%+v", store.cfg, d.commands)
	}
}

func TestClassifySetupKey(t *testing.T) {
	for _, tc := range []struct {
		vk, ch uint16
		ctrl   bool
		want   SetupKey
	}{
		{0x1B, 0x1B, false, KeyQuit},
		{0x43, 0x03, true, KeyQuit},
		{0x43, 'c', false, KeyOther},
		{0x20, ' ', false, KeySpace},
		{0x0D, '\r', false, KeyEnter},
		{0x41, 'a', false, KeyOther},
	} {
		if got := ClassifySetupKey(tc.vk, tc.ch, tc.ctrl); got != tc.want {
			t.Errorf("ClassifySetupKey(%#x, %#x, %v) = %v, want %v", tc.vk, tc.ch, tc.ctrl, got, tc.want)
		}
	}
}
