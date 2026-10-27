package systemd

import "testing"

func TestSourceControlJobRepliesDoNotGeneralizeReadOnlyConnections(t *testing.T) {
	for _, object := range []string{"/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/job/4294967295"} {
		frame := sourceObjectReplyVector(":1.4", object)
		if err := verifyManagerFrame(frame, ":1.4", true, true, sourceUnitReply); err != nil {
			t.Fatal("canonical fixed-control job reply refused", object, err)
		}
		if verifyManagerFrame(frame, ":1.4", true, false, sourceUnitReply) == nil || verifyManagerFrame(frame, ":1.4", false, true, sourceUnitReply) == nil {
			t.Fatal("job vocabulary leaked into read-only/scalar connection")
		}
		for length := 0; length < len(frame); length++ {
			if verifyManagerFrame(frame[:length], ":1.4", true, true, sourceUnitReply) == nil {
				t.Fatal("partial control frame accepted", length)
			}
		}
		if verifyManagerFrame(sourceObjectReplyVector(":1.5", object), ":1.4", true, true, sourceUnitReply) == nil {
			t.Fatal("foreign manager job reply accepted")
		}
	}
	for _, object := range []string{"/org/freedesktop/systemd1/job/0", "/org/freedesktop/systemd1/job/01", "/org/freedesktop/systemd1/job/+1", "/org/freedesktop/systemd1/job/-1", "/org/freedesktop/systemd1/job/4294967296", "/org/freedesktop/systemd1/job/1/extra", "/org/freedesktop/systemd1/unit/ardents_2dendpoint_2eservice"} {
		if verifyManagerFrame(sourceObjectReplyVector(":1.4", object), ":1.4", true, true, sourceUnitReply) == nil {
			t.Fatal("foreign or noncanonical job object accepted", object)
		}
	}
}
