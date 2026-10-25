package core

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestHandlePowerEventFollowsConfig(t *testing.T) {
	for _, tc := range []struct {
		event PowerEvent
		cfg   Config
		want  []string
	}{
		{Startup, Config{Input: "2.0.0.0", TurnOn: TurnOnEvents{Startup: true}}, []string{"on", "report", "active"}},
		{Startup, Config{Input: "2.0.0.0", TurnOn: TurnOnEvents{Wake: true}}, nil},
		{Wake, Config{Input: "2.0.0.0", TurnOn: TurnOnEvents{Wake: true}}, []string{"on", "report", "active"}},
		{Sleep, Config{Input: "2.0.0.0", TurnOff: TurnOffEvents{Sleep: true}}, []string{"report", "off"}},
		{Sleep, Config{Input: "2.0.0.0", TurnOff: TurnOffEvents{Shutdown: true}}, nil},
		{Shutdown, Config{TurnOff: TurnOffEvents{Shutdown: true}}, []string{"off"}},
	} {
		d := &fakeDevice{}
		c, _ := NewController(d, &memoryStore{tc.cfg}, nil)
		c.sleep = func(context.Context, time.Duration) error { return nil }
		if err := c.HandlePowerEvent(context.Background(), tc.event, ""); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(time.Second)
		for len(d.names()) < len(tc.want) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		time.Sleep(5 * time.Millisecond)
		if got := d.names(); len(got)+len(tc.want) > 0 && !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s with %+v: commands = %v, want %v", tc.event, tc.cfg, got, tc.want)
		}
	}
}

func TestSetConfigSavesWithoutCEC(t *testing.T) {
	d, store := &fakeDevice{}, &memoryStore{cfg: DefaultConfig()}
	c, _ := NewController(d, store, nil)
	saved, err := c.SetConfig(Config{Input: "hdmi4", TurnOn: TurnOnEvents{Wake: true}})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Input: "4.0.0.0", InputDelayMS: DefaultInputDelayMS, TurnOn: TurnOnEvents{Wake: true}}
	if saved != want || store.cfg != want || c.Config() != want || len(d.commands) != 0 {
		t.Fatalf("saved=%+v store=%+v commands=%v", saved, store.cfg, d.names())
	}
	if _, err := c.SetConfig(Config{Input: "HDMI9"}); err == nil || store.cfg != want {
		t.Fatalf("invalid input accepted: err=%v store=%+v", err, store.cfg)
	}
}

func TestWakeAfterQuickSleepTurnsTVOnAgain(t *testing.T) {
	d := &fakeDevice{}
	c, _ := NewController(d, &memoryStore{DefaultConfig()}, nil)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	waitIdle := func() {
		for deadline := time.Now().Add(time.Second); c.Status().AutoActive && time.Now().Before(deadline); {
			time.Sleep(time.Millisecond)
		}
	}
	for _, event := range []PowerEvent{Wake, Sleep, Wake, Wake} {
		if err := c.HandlePowerEvent(context.Background(), event, ""); err != nil {
			t.Fatal(err)
		}
		waitIdle()
	}
	if got, want := d.names(), []string{"on", "off", "on"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}
