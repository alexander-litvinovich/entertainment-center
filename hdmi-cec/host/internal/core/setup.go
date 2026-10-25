package core

// SetupInputs is the scan order used by interactive input setup.
var SetupInputs = []string{"HDMI1", "HDMI2", "HDMI3", "HDMI4"}

// NextSetupInput returns the index after current (-1 before the first step)
// and whether the scan wrapped past the end of the list.
func NextSetupInput(current int) (next int, wrapped bool) {
	if current >= len(SetupInputs)-1 {
		return 0, current >= 0
	}
	return current + 1, false
}

type SetupKey int

const (
	KeyOther SetupKey = iota
	KeySpace
	KeyEnter
	KeyQuit
)

// ClassifySetupKey maps a console key-down event (virtual key, character, Ctrl held) to a setup action.
func ClassifySetupKey(virtualKey, char uint16, ctrl bool) SetupKey {
	switch {
	case virtualKey == 0x1B, ctrl && (char == 0x03 || virtualKey == 'C'):
		return KeyQuit
	case virtualKey == 0x20:
		return KeySpace
	case virtualKey == 0x0D:
		return KeyEnter
	}
	return KeyOther
}
