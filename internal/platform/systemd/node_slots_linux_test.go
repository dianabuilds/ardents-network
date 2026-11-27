package systemd

import (
	"bytes"
	"testing"
)

func TestNodeSlotsReplyGrammarRetainsOriginalManagerScalars(t *testing.T) {
	// D-Bus strings and object paths share the same length-prefixed body. Use
	// the independent existing encoder, changing only the header signature.
	frame := managerObjectReplyVector(":1.4", "original-manager-observation")
	object := []byte{8, 1, 'g', 0, 1, 'o', 0}
	scalar := []byte{8, 1, 'g', 0, 1, 's', 0}
	if bytes.Count(frame, object) != 1 {
		t.Fatal("independent scalar fixture header changed")
	}
	frame = bytes.Replace(frame, object, scalar, 1)
	if err := verifyManagerFrame(frame, ":1.4", true, nodeSlotsReply); err != nil {
		t.Fatal("absence-only client lost genuine manager/broker scalar replies", err)
	}
	if verifyManagerFrame(frame, ":1.5", true, nodeSlotsReply) == nil ||
		verifyManagerFrame(frame, ":1.4", true, managerUnitReply(255)) == nil {
		t.Fatal("foreign manager or unknown vocabulary supplied an observation")
	}
}
