package connection

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

func TestReplacementPreflightPreservesPendingByteState(t *testing.T) {
	t.Parallel()
	send, _ := newSendLedger(1, 100)
	if err := send.accept([]byte("request")); err != nil {
		t.Fatal(err)
	}
	pending, err := send.beginWrite()
	if err != nil {
		t.Fatal(err)
	}
	if err := send.acknowledge(1, 2); err != nil {
		t.Fatal(err)
	}
	send.closeInput()
	before := *send
	before.data = bytes.Clone(send.data)
	for _, attempt := range []struct{ generation, next uint64 }{{2, 4}, {1, 4}, {3, 4}, {2, 1}, {2, 8}} {
		err := send.checkReplacement(attempt.generation, attempt.next)
		valid := attempt.generation == 2 && attempt.next == 4
		if (err == nil) != valid {
			t.Fatal("send preflight accepted wrong authenticated range", attempt, err)
		}
		if !reflect.DeepEqual(before, *send) || pending.consumed {
			t.Fatal("preflight trimmed replay or retired the original writer")
		}
	}
	// A still active old writer completes against its original generation.
	if current, err := send.finishWrite(pending, nil); !current || err != nil || send.next != 7 {
		t.Fatal("preflight changed old writer completion", err)
	}

	receive, _ := newReceiveLedger(1, 100)
	if err := receive.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("response")}); err != nil {
		t.Fatal(err)
	}
	if n, err := receive.read(make([]byte, 3)); n != 3 || err != nil {
		t.Fatal(err)
	}
	if err := receive.closeInput(wire.Terminal{AttachmentGeneration: 1, Offset: 8}); err != nil {
		t.Fatal(err)
	}
	prior := *receive
	prior.data = bytes.Clone(receive.data)
	for _, attempt := range []struct{ generation, base, end uint64 }{{2, 3, 8}, {1, 3, 8}, {3, 3, 8}, {2, 9, 9}, {2, 3, 7}, {2, 3, 9}} {
		err := receive.checkReplacement(attempt.generation, attempt.base, attempt.end)
		valid := attempt.generation == 2 && attempt.base == 3 && attempt.end == 8
		if (err == nil) != valid {
			t.Fatal("receive preflight accepted wrong retained range", attempt, err)
		}
		if !reflect.DeepEqual(prior, *receive) {
			t.Fatal("preflight changed receive cursor, EOF or comparison bytes")
		}
	}
}
