package systemd

import "testing"

func TestForwardingControlJobVocabularyIsSeparateFromReadOnlyAndSource(t *testing.T) {
	for _, path := range []string{"/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/job/4294967295"} {
		frame := sourceObjectReplyVector(":1.4", path) // shared independent D-Bus byte encoder only
		if err := verifyForwardingManagerFrame(frame, ":1.4", true, true); err != nil {
			t.Fatal("canonical Forwarding root job grammar refused", err)
		}
		if verifyForwardingManagerFrame(frame, ":1.4", true, false) == nil || verifyForwardingManagerFrame(frame, ":1.4", false, true) == nil {
			t.Fatal("Forwarding jobs leaked into read-only/scalar grammar")
		}
		if verifyManagerFrame(frame, ":1.4", true, true, forwardingUnitReply) == nil {
			t.Fatal("Source job flag admitted Forwarding jobs")
		}
		for n := 0; n < len(frame); n++ {
			if verifyForwardingManagerFrame(frame[:n], ":1.4", true, true) == nil {
				t.Fatal("partial Forwarding job frame accepted", n)
			}
		}
		if verifyForwardingManagerFrame(sourceObjectReplyVector(":1.5", path), ":1.4", true, true) == nil {
			t.Fatal("foreign manager supplied Forwarding job frame")
		}
	}
	for _, path := range []string{"/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/01", "/org/freedesktop/systemd1/job/-1", "/org/freedesktop/systemd1/job/4294967296", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"} {
		if verifyForwardingManagerFrame(sourceObjectReplyVector(":1.4", path), ":1.4", true, true) == nil {
			t.Fatal("foreign/noncanonical Forwarding object accepted", path)
		}
	}
}
