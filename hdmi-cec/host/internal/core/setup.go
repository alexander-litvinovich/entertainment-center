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
