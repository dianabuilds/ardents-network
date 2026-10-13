package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

func TestTailReaderDistinguishesPartialServiceRecordFromEmptyTLSTail(t *testing.T) {
	prefix := []byte("ardents-service-connection-v3\x00")
	for size := 0; size < len(prefix); size++ {
		read := observedRecordRead{reader: bytes.NewReader(prefix[:size])}
		_, err := wire.Read(&read)
		if read.received != size || terminalTruncation(err, read.received) != (size == 0) {
			t.Fatal("actual partial record lost plaintext/physical error", size, read.received, err)
		}
	}
}

// Error classification grants no completion: the caller must separately have
// all authenticated Terminal proofs and exact decoded Route-child evidence.
func TestTerminalTruncationCannotHideAnotherFailure(t *testing.T) {
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF, errors.Join(io.EOF, io.ErrUnexpectedEOF), errors.Join(errors.Join(io.ErrUnexpectedEOF))} {
		if !terminalTruncation(err, 0) {
			t.Fatal("sole physical truncation refused", err)
		}
	}
	for _, err := range []error{nil, context.Canceled, net.ErrClosed, errIntegrity, wire.ErrInvalidRecord,
		fmt.Errorf("unclassified wrapper: %w", io.EOF),
		errors.Join(io.EOF, context.Canceled), errors.Join(io.ErrUnexpectedEOF, errIntegrity),
		errors.Join(io.EOF, errors.Join(io.ErrUnexpectedEOF, wire.ErrInvalidRecord))} {
		if terminalTruncation(err, 0) {
			t.Fatal("retained failure classified as sole TLS truncation", err)
		}
	}
}

// These real ledger transitions isolate the current-generation control rule.
// They grant neither live authority nor a decoded Route CLOSE witness.
func TestTerminalTailReplacementRequiresCurrentControlProofs(t *testing.T) {
	x := preparedTerminals(t)
	completeTerminalExchange(t, x)
	stream := &Stream{settled: true, terminal: x, send: &sendLedger{base: 7}}
	if !stream.terminalTailCompleteLocked() {
		t.Fatal("complete original controls did not permit the separate tail check")
	}
	if err := x.replace(2); err != nil {
		t.Fatal(err)
	}
	checkIncomplete := func(boundary string) {
		t.Helper()
		if stream.terminalTailCompleteLocked() {
			t.Fatal("old Application settlement retired incomplete replacement controls", boundary)
		}
	}
	checkIncomplete("replacement")
	terminal, err := x.beginWrite(7)
	if err != nil {
		t.Fatal(err)
	}
	checkIncomplete("Terminal in flight")
	finishControl(t, x, terminal)
	checkIncomplete("local Terminal only")
	first, err := x.receiveTerminal(wire.Terminal{AttachmentGeneration: 2, Offset: 9}, 9, false)
	if err != nil || first {
		t.Fatal("replacement peer Terminal produced another EOF", first, err)
	}
	receipt, err := x.beginWrite(7)
	if err != nil {
		t.Fatal(err)
	}
	checkIncomplete("receipt in flight")
	finishControl(t, x, receipt)
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 2, Offset: 7, Terminal: true}); err != nil {
		t.Fatal(err)
	}
	confirmation, err := x.beginWrite(7)
	if err != nil {
		t.Fatal(err)
	}
	checkIncomplete("confirmation in flight")
	finishControl(t, x, confirmation)
	checkIncomplete("peer confirmation missing")
	if err := x.receiveAcknowledgement(wire.Acknowledgement{AttachmentGeneration: 2, Offset: 9, Terminal: true, TerminalConfirmation: true}); err != nil {
		t.Fatal(err)
	}
	if !stream.terminalTailCompleteLocked() || !stream.settled {
		t.Fatal("complete replacement controls lost original settlement")
	}
}
