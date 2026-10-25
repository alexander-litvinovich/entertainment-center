package tui

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/device"
	"hdmi-cec-host/internal/ipc"
)

type recorder struct {
	handler  ipc.Handler
	requests []ipc.Request
}

func newRecorder(t *testing.T, cfg *core.Config) (*recorder, core.FileConfigStore) {
	store := core.FileConfigStore{Path: filepath.Join(t.TempDir(), "config.json")}
	if cfg != nil {
		if err := store.Save(*cfg); err != nil {
			t.Fatal(err)
		}
	}
	controller, err := core.NewController(device.DryRun{}, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &recorder{handler: ipc.Handler{Controller: controller}}, store
}

func (r *recorder) send(req ipc.Request) (ipc.Response, error) {
	r.requests = append(r.requests, req)
	return r.handler.Handle(context.Background(), req), nil
}

func (r *recorder) start(t *testing.T) setup {
	response, _ := r.send(ipc.Request{Command: "config"})
	return newSetup(*response.Config, r.send, "")
}

var keyCodes = map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "space": tea.KeySpace,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight, "tab": tea.KeyTab}

// press feeds keys and runs the resulting commands, like the Bubble Tea runtime.
func press(t *testing.T, m setup, keys ...string) setup {
	for _, key := range keys {
		code, ok := keyCodes[key]
		if !ok {
			code = rune(key[0])
		}
		next, cmd := m.Update(tea.KeyPressMsg{Code: code})
		m = next.(setup)
		for cmd != nil {
			msg := cmd()
			if batch, ok := msg.(tea.BatchMsg); ok {
				msg = batch[len(batch)-1]()
			}
			if _, quit := msg.(tea.QuitMsg); quit {
				break
			}
			next, cmd = m.Update(msg)
			m = next.(setup)
		}
	}
	return m
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func view(m setup) string { return ansi.ReplaceAllString(m.View().Content, "") }

func TestOnboardingEnterNumber(t *testing.T) {
	r, store := newRecorder(t, nil)
	m := r.start(t)
	if m.screen != welcomeScreen || !strings.Contains(view(m), "No video input is saved yet") {
		t.Fatalf("first screen:\n%s", view(m))
	}
	m = press(t, m, "enter", "right", "right", "down", "space", "down", "down", "space", "down", "down", "enter")
	if m.screen != doneScreen || !m.saved {
		t.Fatalf("not saved: screen %d\n%s", m.screen, view(m))
	}
	cfg, _ := store.Load()
	want := core.DefaultConfig()
	want.Input, want.TurnOn.Wake, want.TurnOff.Sleep = "3.0.0.0", false, false
	if cfg != want {
		t.Fatalf("saved %+v, want %+v", cfg, want)
	}
	if !strings.Contains(view(m), "Saved.\nVideo input: 3 (3.0.0.0)\nTurn TV on: startup\nTurn TV off: shutdown") {
		t.Fatalf("final message:\n%s", view(m))
	}
}

func TestOnboardingDetect(t *testing.T) {
	r, store := newRecorder(t, nil)
	m := press(t, r.start(t), "down", "enter")
	if m.screen != detectScreen || !strings.Contains(view(m), "When the TV shows this PC, press Enter") {
		t.Fatalf("detect screen:\n%s", view(m))
	}
	m = press(t, m, "enter")
	if m.screen != detectScreen || !strings.Contains(view(m), "Press Space first") {
		t.Fatalf("enter before space:\n%s", view(m))
	}
	m = press(t, m, "o", "space", "space")
	if !strings.Contains(view(m), "HDMI2: sent (CEC result: ack)") {
		t.Fatalf("after two probes:\n%s", view(m))
	}
	m = press(t, m, "enter")
	if m.screen != formScreen || m.focus != rowSave || !strings.Contains(view(m), "Video input: < 2 >") {
		t.Fatalf("form after detection:\n%s", view(m))
	}
	if cfg, _ := store.Load(); cfg.Input != "" {
		t.Fatalf("detection saved before Save: %+v", cfg)
	}
	m = press(t, m, "enter")
	if cfg, _ := store.Load(); !m.saved || cfg.Input != "2.0.0.0" {
		t.Fatalf("saved=%v cfg=%+v", m.saved, cfg)
	}
	var sent []string
	for _, req := range r.requests {
		sent = append(sent, req.Command+" "+req.Input)
	}
	if got, want := strings.Join(sent, ","), "config ,send 40:04,probe HDMI1,probe HDMI2,config "; got != want {
		t.Fatalf("requests %q, want %q", got, want)
	}
}

func TestDetectWrapsAndEscReturns(t *testing.T) {
	r, _ := newRecorder(t, nil)
	m := press(t, r.start(t), "down", "enter", "space", "space", "space", "space", "space")
	if m.probed != 0 || !strings.Contains(view(m), "HDMI1: sent") {
		t.Fatalf("probed %d:\n%s", m.probed, view(m))
	}
	if m = press(t, m, "esc"); m.screen != welcomeScreen {
		t.Fatalf("esc went to screen %d", m.screen)
	}
}

func TestSettingsFormMatchesLayout(t *testing.T) {
	saved := core.DefaultConfig()
	saved.Input = "2.0.0.0"
	r, _ := newRecorder(t, &saved)
	m := r.start(t)
	want := "HDMI-CEC Setup\n\nVideo input: < 2 >\n\nTurn TV on:\n[x] Wake\n[x] Startup\n\nTurn TV off:\n[x] Sleep\n[x] Shutdown\n\nSave   Cancel\n"
	if got := view(m); !strings.HasPrefix(got, want) {
		t.Fatalf("form:\n%s\nwant prefix:\n%s", got, want)
	}
}

func TestCancelKeepsConfig(t *testing.T) {
	saved := core.DefaultConfig()
	saved.Input = "2.1.0.0"
	r, store := newRecorder(t, &saved)
	m := r.start(t)
	if !strings.Contains(view(m), "Video input: < 2.1.0.0 >") {
		t.Fatalf("custom address:\n%s", view(m))
	}
	if m = press(t, m, "d"); m.screen != formScreen {
		t.Fatalf("d left the form for screen %d", m.screen)
	}
	m = press(t, m, "1", "down", "space", "esc")
	if cfg, _ := store.Load(); m.saved || m.screen != doneScreen || cfg != saved {
		t.Fatalf("saved=%v cfg=%+v", m.saved, cfg)
	}
	if !strings.Contains(view(m), "Cancelled") {
		t.Fatalf("final:\n%s", view(m))
	}
}
