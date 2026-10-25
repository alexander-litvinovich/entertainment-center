// Package tui holds the interactive terminal screens of the hdmi-cec CLI.
package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"hdmi-cec-host/internal/core"
	"hdmi-cec-host/internal/ipc"
)

// Send delivers a request to the host: the service over IPC, or an in-process dry-run controller.
type Send func(ipc.Request) (ipc.Response, error)

// RunSetup loads the host's config and shows onboarding (no input saved yet) or the settings form.
// It reports whether the user saved.
func RunSetup(send Send, note string) (bool, error) {
	response, err := send(ipc.Request{Command: "config"})
	if err != nil {
		return false, err
	}
	if !response.OK || response.Config == nil {
		if response.Message == "unknown command" {
			return false, errors.New("the running host is older than this CLI: update the service (hdmi-cec update) or re-run the installer")
		}
		return false, errors.New(response.Message)
	}
	final, err := tea.NewProgram(newSetup(*response.Config, send, note)).Run()
	if err != nil {
		return false, err
	}
	return final.(setup).saved, nil
}

type screen int

const (
	welcomeScreen screen = iota
	detectScreen
	formScreen
	doneScreen
)

// Form rows in focus order.
const (
	rowInput = iota
	rowWake
	rowStartup
	rowSleep
	rowShutdown
	rowSave
	rowCancel
	rowCount
)

type setup struct {
	send Send
	note string

	screen screen
	cfg    core.Config
	// inputs holds the physical addresses offered for the video input: HDMI1..HDMI4 in
	// core.SetupInputs order, then a saved address that is not one of them.
	inputs  []string
	input   int
	focus   int
	choice  int // welcome screen: 0 enter the number, 1 detect
	probed  int // detection: index into core.SetupInputs on the TV, -1 before the first Space
	busy    bool
	status  string
	saved   bool
	message string
}

type probeMsg struct {
	name     string
	response ipc.Response
	err      error
}

type powerOnMsg struct {
	response ipc.Response
	err      error
}

type saveMsg struct {
	response ipc.Response
	err      error
}

func newSetup(cfg core.Config, send Send, note string) setup {
	m := setup{send: send, note: note, cfg: cfg, probed: -1, input: -1}
	for i, name := range core.SetupInputs {
		_, address, _ := core.ParseAddress(name)
		m.inputs = append(m.inputs, address)
		if address == cfg.Input {
			m.input = i
		}
	}
	switch {
	case cfg.Input == "":
		m.input, m.screen = 0, welcomeScreen
	case m.input < 0:
		m.inputs = append(m.inputs, cfg.Input)
		m.input, m.screen = len(m.inputs)-1, formScreen
	default:
		m.screen = formScreen
	}
	return m
}

func (m setup) Init() tea.Cmd { return nil }

func (m setup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Keystroke uses the US-layout key, so letters work with any keyboard layout.
		key := msg.Keystroke()
		if key == "ctrl+c" {
			return m.finish(false, "Cancelled; settings unchanged.")
		}
		if m.busy {
			return m, nil
		}
		switch m.screen {
		case welcomeScreen:
			return m.welcomeKey(key)
		case detectScreen:
			return m.detectKey(key)
		case formScreen:
			return m.formKey(key)
		}
	case probeMsg:
		m.busy = false
		m.status = result(msg.name, msg.response, msg.err) + " Does the TV show this PC?"
	case powerOnMsg:
		m.busy = false
		m.status = result("Power On", msg.response, msg.err)
	case saveMsg:
		m.busy = false
		if msg.err == nil && msg.response.OK {
			return m.finish(true, "Saved.\n"+describe(*msg.response.Config))
		}
		m.status = result("Save", msg.response, msg.err)
	}
	return m, nil
}

func result(action string, response ipc.Response, err error) string {
	switch {
	case err != nil:
		return action + ": host error: " + err.Error()
	case !response.OK:
		return action + " failed: " + response.Message
	case response.Status == nil:
		return action + ": done."
	}
	return fmt.Sprintf("%s: sent (CEC result: %s).", action, response.Status.LastResult)
}

func (m setup) welcomeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "down", "k", "j", "tab", "shift+tab":
		m.choice = 1 - m.choice
	case "1", "2", "3", "4":
		m.input = int(key[0] - '1')
		m.screen, m.focus = formScreen, rowSave
	case "enter", "space":
		if m.choice == 1 {
			m.screen, m.probed, m.status = detectScreen, -1, ""
			return m, nil
		}
		m.screen, m.focus = formScreen, rowInput
	case "esc", "q":
		return m.finish(false, "Cancelled; settings unchanged. Run `hdmi-cec setup` later.")
	}
	return m, nil
}

func (m setup) detectKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "space":
		next, wrapped := core.NextSetupInput(m.probed)
		m.probed = next
		name := core.SetupInputs[next]
		m.busy, m.status = true, "Switching the TV to "+name+"…"
		probe := func() tea.Msg {
			response, err := m.send(ipc.Request{Command: "probe", Input: name})
			return probeMsg{name, response, err}
		}
		if wrapped {
			m.status = "Back to the first input. " + m.status
			return m, tea.Batch(tea.Raw("\a"), probe)
		}
		return m, probe
	case "enter":
		if m.probed < 0 {
			m.status = "Press Space first to switch the TV to HDMI1."
			return m, nil
		}
		m.input, m.screen, m.focus = m.probed, formScreen, rowSave
		m.status = "Detected " + core.SetupInputs[m.probed] + ". Choose Save to keep it."
	case "o":
		m.busy, m.status = true, "Sending Power On…"
		return m, func() tea.Msg {
			response, err := m.send(ipc.Request{Command: "send", Input: core.FormatFrame(core.FrameOn())})
			return powerOnMsg{response, err}
		}
	case "esc", "q":
		m.screen, m.status = welcomeScreen, ""
	}
	return m, nil
}

func (m setup) formKey(key string) (tea.Model, tea.Cmd) {
	checkboxes := map[int]*bool{
		rowWake: &m.cfg.TurnOn.Wake, rowStartup: &m.cfg.TurnOn.Startup,
		rowSleep: &m.cfg.TurnOff.Sleep, rowShutdown: &m.cfg.TurnOff.Shutdown,
	}
	switch key {
	case "up", "k", "shift+tab":
		m.focus = (m.focus + rowCount - 1) % rowCount
	case "down", "j", "tab":
		m.focus = (m.focus + 1) % rowCount
	case "left", "h", "right", "l":
		step := 1
		if key == "left" || key == "h" {
			step = len(m.inputs) - 1
		}
		switch m.focus {
		case rowInput:
			m.input = (m.input + step) % len(m.inputs)
		case rowSave, rowCancel:
			m.focus = rowSave + rowCancel - m.focus
		}
	case "1", "2", "3", "4":
		if m.focus == rowInput {
			m.input = int(key[0] - '1')
		}
	case "space", "x", "enter":
		if value, ok := checkboxes[m.focus]; ok {
			*value = !*value
			return m, nil
		}
		switch m.focus {
		case rowInput:
			m.focus = rowWake
		case rowSave:
			return m.save()
		case rowCancel:
			return m.finish(false, "Cancelled; settings unchanged.")
		}
	case "esc", "q":
		return m.finish(false, "Cancelled; settings unchanged.")
	}
	return m, nil
}

func (m setup) save() (tea.Model, tea.Cmd) {
	cfg := m.cfg
	cfg.Input = m.inputs[m.input]
	m.busy, m.status = true, "Saving…"
	return m, func() tea.Msg {
		response, err := m.send(ipc.Request{Command: "config", Config: &cfg})
		return saveMsg{response, err}
	}
}

func (m setup) finish(saved bool, message string) (tea.Model, tea.Cmd) {
	m.screen, m.saved, m.message = doneScreen, saved, message
	return m, tea.Quit
}

func (m setup) View() tea.View {
	if m.screen == doneScreen {
		return tea.NewView(m.message + "\n")
	}
	var b strings.Builder
	b.WriteString(bold("HDMI-CEC Setup"))
	if m.note != "" {
		b.WriteString("  " + dim(m.note))
	}
	b.WriteString("\n\n")
	var help string
	switch m.screen {
	case welcomeScreen:
		b.WriteString("No video input is saved yet. Which TV input is this PC connected to?\n\n")
		for i, label := range []string{"Enter the input number", "Detect it: switch the TV through its inputs"} {
			marker := "  "
			if i == m.choice {
				marker = "> "
			}
			b.WriteString(marker + highlight(label, i == m.choice) + "\n")
		}
		help = "↑/↓ choose · Enter confirm · 1-4 input number · Esc quit"
	case detectScreen:
		b.WriteString("Detect the video input\n\n")
		b.WriteString("1. Turn the TV on (press O to send Power On over CEC).\n")
		b.WriteString("2. Press Space: the TV switches to HDMI1. Each Space moves on to the next\n")
		b.WriteString("   input; after HDMI4 a beep marks the start of the list again.\n")
		b.WriteString("3. When the TV shows this PC, press Enter.\n")
		help = "Space next input · Enter this one · O TV on · Esc back"
	case formScreen:
		b.WriteString("Video input: " + highlight("< "+inputLabel(m.inputs[m.input])+" >", m.focus == rowInput) + "\n\n")
		b.WriteString("Turn TV on:\n")
		b.WriteString(highlight(checkbox(m.cfg.TurnOn.Wake, "Wake"), m.focus == rowWake) + "\n")
		b.WriteString(highlight(checkbox(m.cfg.TurnOn.Startup, "Startup"), m.focus == rowStartup) + "\n\n")
		b.WriteString("Turn TV off:\n")
		b.WriteString(highlight(checkbox(m.cfg.TurnOff.Sleep, "Sleep"), m.focus == rowSleep) + "\n")
		b.WriteString(highlight(checkbox(m.cfg.TurnOff.Shutdown, "Shutdown"), m.focus == rowShutdown) + "\n\n")
		b.WriteString(highlight("Save", m.focus == rowSave) + "   " + highlight("Cancel", m.focus == rowCancel) + "\n")
		help = "↑/↓ move · ←/→ or 1-4 input · Space toggle · Enter select · Esc cancel"
	}
	if m.status != "" {
		b.WriteString("\n" + m.status + "\n")
	}
	b.WriteString("\n" + dim(help) + "\n")
	return tea.NewView(b.String())
}

// inputLabel shows an HDMI port address such as 2.0.0.0 as the port number.
func inputLabel(address string) string {
	if len(address) == 7 && strings.HasSuffix(address, ".0.0.0") && address[0] >= '1' && address[0] <= '9' {
		return address[:1]
	}
	return address
}

func describe(cfg core.Config) string {
	events := func(names []string, enabled ...bool) string {
		var out []string
		for i, name := range names {
			if enabled[i] {
				out = append(out, name)
			}
		}
		if len(out) == 0 {
			return "never"
		}
		return strings.Join(out, ", ")
	}
	return fmt.Sprintf("Video input: %s (%s)\nTurn TV on: %s\nTurn TV off: %s", inputLabel(cfg.Input), cfg.Input,
		events([]string{"wake", "startup"}, cfg.TurnOn.Wake, cfg.TurnOn.Startup),
		events([]string{"sleep", "shutdown"}, cfg.TurnOff.Sleep, cfg.TurnOff.Shutdown))
}

func checkbox(on bool, label string) string {
	if on {
		return "[x] " + label
	}
	return "[ ] " + label
}

func highlight(s string, on bool) string {
	if on {
		return "\x1b[7m" + s + "\x1b[27m"
	}
	return s
}

func bold(s string) string { return "\x1b[1m" + s + "\x1b[22m" }
func dim(s string) string  { return "\x1b[2m" + s + "\x1b[22m" }
