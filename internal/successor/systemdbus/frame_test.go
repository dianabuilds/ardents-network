package systemdbus

import "testing"

func TestFramePolicyRefusesInvalidInputsAndNoncanonicalJob(t *testing.T) {
	for _, p := range []FramePolicy{{AllowObject: true}, {AllowJob: true}} {
		if err := ValidateFrame(make([]byte, 16), p); err == nil {
			t.Fatal("incomplete object policy accepted")
		}
	}
	for _, path := range []string{"", "/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/01", "/org/freedesktop/systemd1/job/4294967296", "/other/1"} {
		if ValidJobPath(path) {
			t.Fatalf("foreign or noncanonical job accepted: %q", path)
		}
	}
	if !ValidJobPath("/org/freedesktop/systemd1/job/42") {
		t.Fatal("canonical job refused")
	}
}
