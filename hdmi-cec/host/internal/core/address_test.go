package core

import "testing"

func TestParseAddress(t *testing.T) {
	tests := map[string]string{"HDMI1": "1.0.0.0", "hdmi4": "4.0.0.0", "2.1.0.0": "2.1.0.0", "A.B.0.0": "A.B.0.0"}
	for input, want := range tests {
		_, got, err := ParseAddress(input)
		if err != nil || got != want {
			t.Fatalf("ParseAddress(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"HDMI0", "HDMI5", "0.0.0.0", "1.0.2.0", "1.2.0.3", "12.0.0.0"} {
		if _, _, err := ParseAddress(input); err == nil {
			t.Errorf("ParseAddress(%q) unexpectedly succeeded", input)
		}
	}
}
