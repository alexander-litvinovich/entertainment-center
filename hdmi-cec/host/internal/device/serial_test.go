package device

import "testing"

func TestParseResponseUsesMatchingID(t *testing.T) {
	state, result, ok := ParseResponse("@cec id=7 state=done result=nack\r\n@cec id=8 state=queued", 7)
	if !ok || state != "done" || result != "nack" {
		t.Fatalf("got %q %q %v", state, result, ok)
	}
	if _, _, ok := ParseResponse("@cec id=8 state=done result=ack", 7); ok {
		t.Fatal("accepted another request ID")
	}
}

func TestVersionPattern(t *testing.T) {
	for _, version := range []string{"cec-host-0.6.0", "cec-host-0.6.0-deadbeef"} {
		if !versionRE.MatchString(version) {
			t.Errorf("did not match %q", version)
		}
	}
	if versionRE.MatchString("cec-wake-0.5.0") {
		t.Fatal("accepted old protocol")
	}
}
