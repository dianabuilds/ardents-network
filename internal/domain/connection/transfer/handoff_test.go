package transfer

import (
	"bytes"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// Pure ledgers supply no live permission. Planning a transition must not spend
// the Application cursor or expose pending bytes to a physical writer.
func TestByteHandoffPlanningLeavesOriginalLedgersUnchanged(t *testing.T) {
	received, err := newReceiveLedger(1, 64)
	if err != nil {
		t.Fatal(err)
	}
	if err := received.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("retained")}); err != nil {
		t.Fatal(err)
	}
	staged := make([]byte, 8)
	for range 2 {
		if n, err := received.peek(staged); n != 8 || err != nil || received.readAt != 0 || string(staged) != "retained" {
			t.Fatal("planning consumed original Application bytes", n, err, received.readAt)
		}
	}
	if n, err := received.read(staged); n != 8 || err != nil || received.readAt != 8 {
		t.Fatal("accepted delivery did not consume once", n, err, received.readAt)
	}
	sent, err := newSendLedger(1, 64)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := sent.checkAccept(staged); err != nil || sent.end != 0 || len(sent.data) != 0 {
			t.Fatal("planning exposed uncommitted output bytes", err, sent.end)
		}
	}
	if err := sent.accept(staged); err != nil || sent.end != 8 || !bytes.Equal(sent.data, staged) {
		t.Fatal("accepted output did not commit once", err, sent.end)
	}
}
