package connection

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

func TestReceiveReplayAcrossReplacementHasOneApplicationEffect(t *testing.T) {
	ledger, err := newReceiveLedger(1, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("abcdef")}); err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 3)
	if n, err := ledger.read(first); n != 3 || err != nil || string(first) != "abc" {
		t.Fatal(n, err, string(first))
	}
	if err := ledger.replace(2, 0, 9); err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 2, Offset: 3, Payload: []byte("defghi")}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 2, Payload: []byte("abcdefghi")}); err != nil {
		t.Fatal(err)
	}
	rest := make([]byte, 20)
	if n, err := ledger.read(rest); n != 6 || err != nil || string(rest[:n]) != "defghi" {
		t.Fatal("replacement repeated or lost Application bytes", n, err, string(rest[:n]))
	}
	if n, err := ledger.read(rest); n != 0 || err != nil {
		t.Fatal("empty active queue supplied EOF", n, err)
	}
	if err := ledger.closeInput(wire.Terminal{AttachmentGeneration: 2, Offset: 9}); err != nil {
		t.Fatal(err)
	}
	if n, err := ledger.read(rest); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("authenticated Terminal did not close its direction", n, err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 2, Offset: 9, Payload: []byte("j")}); !errors.Is(err, errIntegrity) {
		t.Fatal("fresh Data accepted after Terminal", err)
	}
}

func TestReceiveContradictionHasNoQueueOrCounterEffect(t *testing.T) {
	for _, test := range []struct {
		name  string
		value wire.Data
	}{
		{"gap", wire.Data{AttachmentGeneration: 1, Offset: 4, Payload: []byte("e")}},
		{"conflicting replay", wire.Data{AttachmentGeneration: 1, Offset: 1, Payload: []byte("xd")}},
		{"foreign generation", wire.Data{AttachmentGeneration: 2, Offset: 3, Payload: []byte("d")}},
		{"overflow", wire.Data{AttachmentGeneration: 1, Offset: ^uint64(0), Payload: []byte("d")}},
		{"workload bound", wire.Data{AttachmentGeneration: 1, Offset: 3, Payload: []byte("def")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger, _ := newReceiveLedger(1, 5)
			if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("abc")}); err != nil {
				t.Fatal(err)
			}
			if err := ledger.accept(test.value); !errors.Is(err, errIntegrity) {
				t.Fatal("contradiction accepted", err)
			}
			if ledger.next != 3 || ledger.readAt != 0 || string(ledger.data) != "abc" {
				t.Fatal("refusal changed accepted bytes")
			}
		})
	}
}

func TestReceiveCapacityRetainsUnreadBytesAndBoundedReplay(t *testing.T) {
	ledger, _ := newReceiveLedger(1, logicalQueueLimit+2*wire.MaximumDataBytes)
	chunk := bytes.Repeat([]byte{42}, wire.MaximumDataBytes)
	for offset := uint64(0); offset < logicalQueueLimit; offset += uint64(len(chunk)) {
		if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Offset: offset, Payload: chunk}); err != nil {
			t.Fatal(err)
		}
	}
	fresh := wire.Data{AttachmentGeneration: 1, Offset: logicalQueueLimit, Payload: []byte{7}}
	if err := ledger.accept(fresh); !errors.Is(err, errBackpressure) {
		t.Fatal("full unread queue accepted bytes", err)
	}
	if ledger.next != logicalQueueLimit || ledger.base != 0 || len(ledger.data) != logicalQueueLimit || cap(ledger.data) > logicalQueueLimit {
		t.Fatal("capacity refusal changed bounded state")
	}
	consumed := make([]byte, wire.MaximumDataBytes)
	if n, err := ledger.read(consumed); n != len(consumed) || err != nil || !bytes.Equal(consumed, chunk) {
		t.Fatal("capacity evicted unread bytes", n, err)
	}
	if err := ledger.accept(fresh); err != nil {
		t.Fatal("consumption did not allow bounded progress", err)
	}
	if ledger.base != 1 || ledger.readAt != wire.MaximumDataBytes || len(ledger.data) != logicalQueueLimit || cap(ledger.data) > logicalQueueLimit {
		t.Fatal("recent history and pending bytes exceed one bounded buffer")
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte{42}}); !errors.Is(err, errIntegrity) {
		t.Fatal("unverifiable aged replay accepted", err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Offset: 1, Payload: chunk}); err != nil {
		t.Fatal("retained matching replay refused", err)
	}
}

func TestReceiveReplacementCannotResetWorkOrHideMissingRange(t *testing.T) {
	ledger, _ := newReceiveLedger(1, 3)
	if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("abc")}); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range [][3]uint64{{2, 4, 4}, {2, 0, 2}, {3, 0, 3}, {2, 0, 4}} {
		if err := ledger.replace(replacement[0], replacement[1], replacement[2]); !errors.Is(err, errIntegrity) || ledger.generation != 1 {
			t.Fatal("replacement lost original range or work bound", replacement, err)
		}
	}
	if err := ledger.closeInput(wire.Terminal{AttachmentGeneration: 1, Offset: 4}); !errors.Is(err, errIntegrity) || ledger.terminal {
		t.Fatal("Terminal hid a missing range", err)
	}
	if err := ledger.replace(2, 0, 3); err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 2, Offset: 3, Payload: []byte("d")}); !errors.Is(err, errIntegrity) {
		t.Fatal("replacement reset workload counter", err)
	}
}

func TestReceiveReplacementPreservesAuthenticatedTerminal(t *testing.T) {
	ledger, _ := newReceiveLedger(1, 10)
	if err := ledger.accept(wire.Data{AttachmentGeneration: 1, Payload: []byte("abc")}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.closeInput(wire.Terminal{AttachmentGeneration: 1, Offset: 3}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.replace(2, 0, 4); !errors.Is(err, errIntegrity) || ledger.generation != 1 {
		t.Fatal("replacement admitted bytes beyond authenticated Terminal", err)
	}
	if err := ledger.replace(2, 0, 3); err != nil {
		t.Fatal(err)
	}
	if err := ledger.accept(wire.Data{AttachmentGeneration: 2, Payload: []byte("abc")}); err != nil {
		t.Fatal("matching Terminal-prefix replay refused", err)
	}
	output := make([]byte, 10)
	if n, err := ledger.read(output); n != 3 || err != nil || string(output[:n]) != "abc" {
		t.Fatal("Terminal lost previously unread bytes", n, err)
	}
	if n, err := ledger.read(output); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("replacement reopened closed direction", n, err)
	}
}
