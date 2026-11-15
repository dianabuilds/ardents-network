package systemd

import "testing"

// A Introduction job object is legal only on its own original root control after job
// admission. No stopped/read-only purpose borrows any role's mutation right.
func TestIntroductionControlReplyVocabularyIsPurposeAndAdmissionBound(t *testing.T) {
	const manager = ":1.4"
	for _, object := range []string{"/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice", "/org/freedesktop/systemd1/job/1", "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice", "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice", "/org/freedesktop/systemd1/job/01", "/org/freedesktop/systemd1/job/4294967296"} {
		frame := sourceObjectReplyVector(manager, object)
		for _, purpose := range []managerUnitReply{introductionUnitReply, introductionControlReply} {
			for _, enabled := range []bool{false, true} {
				want := object == "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice" || object == "/org/freedesktop/systemd1/job/1" && purpose == introductionControlReply && enabled
				err := verifyNodeManagerFrame(frame, manager, true, true, true, enabled, purpose)
				if (err == nil) != want {
					t.Fatal("Introduction mutation vocabulary escaped its original purpose", object, purpose, enabled, err)
				}
				if verifyNodeManagerFrame(frame, ":1.5", true, true, true, enabled, purpose) == nil {
					t.Fatal("foreign original manager accepted")
				}
			}
		}
	}
}
