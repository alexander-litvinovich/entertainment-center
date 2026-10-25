package core

import (
	"bytes"
	"testing"
)

func TestFrames(t *testing.T) {
	for name, tc := range map[string]struct {
		got  []byte
		want string
	}{
		"on":     {FrameOn(), "40:04"},
		"off":    {FrameOff(), "40:36"},
		"report": {FrameReportPhysicalAddress(0x2100), "4F:84:21:00:04"},
		"active": {FrameActiveSource(0x2000), "4F:82:20:00"},
	} {
		if FormatFrame(tc.got) != tc.want {
			t.Errorf("%s = %s, want %s", name, FormatFrame(tc.got), tc.want)
		}
	}
}

func TestParseFrame(t *testing.T) {
	for _, in := range []string{"4F:82:20:00", "4f 82 20 00", "4F822000"} {
		got, err := ParseFrame(in)
		if err != nil || !bytes.Equal(got, []byte{0x4f, 0x82, 0x20, 0x00}) {
			t.Errorf("ParseFrame(%q) = %x, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "zz", "4F:8", "04:36", "4F:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00"} {
		if _, err := ParseFrame(in); err == nil {
			t.Errorf("ParseFrame(%q) unexpectedly succeeded", in)
		}
	}
}
