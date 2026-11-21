package systemd

import (
	"bytes"
	"testing"
)

func TestNodeSlotsReplyGrammarRefusesEveryUnitAndJobObject(t *testing.T) {
	for _, path := range []string{
		"",
		"/",
		"/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dissuer_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice",
		"/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice",
		"/org/freedesktop/systemd1/job/1",
		"/org/freedesktop/systemd1/job/4294967295",
	} {
		frame := sourceObjectReplyVector(":1.4", path)
		for _, sourceJobs := range []bool{false, true} {
			for _, forwardingJobs := range []bool{false, true} {
				if verifyClosedManagerFrame(frame, ":1.4", true, sourceJobs, forwardingJobs, nodeSlotsReply) == nil {
					t.Fatal("absence-only connection accepted a unit/job object", path)
				}
			}
		}
	}
}

func TestNodeSlotsReplyGrammarRetainsOriginalManagerScalars(t *testing.T) {
	// D-Bus strings and object paths share the same length-prefixed body. Use
	// the independent existing encoder, changing only the header signature.
	frame := sourceObjectReplyVector(":1.4", "original-manager-observation")
	object := []byte{8, 1, 'g', 0, 1, 'o', 0}
	scalar := []byte{8, 1, 'g', 0, 1, 's', 0}
	if bytes.Count(frame, object) != 1 {
		t.Fatal("independent scalar fixture header changed")
	}
	frame = bytes.Replace(frame, object, scalar, 1)
	if err := verifyClosedManagerFrame(frame, ":1.4", true, false, false, nodeSlotsReply); err != nil {
		t.Fatal("absence-only client lost genuine manager/broker scalar replies", err)
	}
	if verifyClosedManagerFrame(frame, ":1.5", true, false, false, nodeSlotsReply) == nil ||
		verifyClosedManagerFrame(frame, ":1.4", true, false, false, managerUnitReply(255)) == nil {
		t.Fatal("foreign manager or unknown vocabulary supplied an observation")
	}
}
