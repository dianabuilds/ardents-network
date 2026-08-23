package serviceconn

import (
	"errors"
	"testing"
)

func TestCutoverRejectsOffsetRollback(t *testing.T) {
	failed := &securedAttachment{generation: 1}
	fresh := &securedAttachment{generation: 2}
	stream := &recoveryStream{current: failed, sendBase: 10, sendEnd: 20, recvNext: 15}
	rollback := peerContinuity{sendBase: 0, sendEnd: 20, recvNext: 9, peerNonce: [32]byte{2}, localNonce: [32]byte{3}}
	if !errors.Is(stream.commitAttachment(failed, fresh, rollback), errActiveViolation) {
		t.Fatal("acknowledgement rollback was accepted")
	}
}
