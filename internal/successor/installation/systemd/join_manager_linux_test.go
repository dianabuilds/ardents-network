package systemd

import "testing"

// The transitional JOIN control vocabulary retired with the orchestration
// (#604 FJD stage D, ADR-0125). The retained read-only JoinRuntime vocabulary
// admits only its own fixed unit object; no job admission flag grants it a
// mutation right, and the shared job admission belongs to Source/Forwarding control.
func TestJoinRuntimeReplyVocabularyIsPurposeAndAdmissionBound(t *testing.T) {
	const manager = ":1.4"
	for _, object := range []string{"/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice", "/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice", "/org/freedesktop/systemd1/job/01", "/org/freedesktop/systemd1/job/4294967296"} {
		frame := sourceObjectReplyVector(manager, object)
		want := object == "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"
		err := verifyNodeManagerFrame(frame, manager, true, true, true, joinUnitReply)
		if (err == nil) != want {
			t.Fatal("JOIN mutation vocabulary escaped its original purpose", object, err)
		}
		if verifyNodeManagerFrame(frame, ":1.5", true, true, true, joinUnitReply) == nil {
			t.Fatal("foreign original manager accepted")
		}
	}
}

func TestJoinRuntimeReplyVocabularyRefusesOtherUnitsAndAllJobs(t *testing.T) {
	for _, object := range []string{"/", "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dissuer_2eservice", "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice", "/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/job/4294967295", "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"} {
		frame := sourceObjectReplyVector(":1.4", object)
		for _, sourceJobs := range []bool{false, true} {
			for _, forwardingJobs := range []bool{false, true} {
				err := verifyClosedManagerFrame(frame, ":1.4", true, sourceJobs, forwardingJobs, joinUnitReply)
				wanted := object == "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"
				if (err == nil) != wanted {
					t.Fatal("fixed read-only JOIN vocabulary changed", object, err)
				}
			}
		}
		if verifyClosedManagerFrame(frame, ":1.5", true, false, false, joinUnitReply) == nil {
			t.Fatal("foreign manager supplied an observation")
		}
	}
}
