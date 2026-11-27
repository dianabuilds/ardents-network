package connection

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/recordio"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// These local retirement states supply no successful authentication or
// Application outcome. They isolate denial before record interruption.
func TestStreamRetirementSealPrecedesLateRecordFailure(t *testing.T) {
	// Deterministically schedule the old cancellation-before-seal window.
	// A settled state alone cannot stop failRecord from retaining cancellation.
	before, cancelBefore := context.WithCancel(t.Context())
	unsealed := &Stream{ctx: before, cancel: cancelBefore, settled: true}
	oldRecords := &attachmentRecords{}
	unsealed.records = oldRecords
	cancelBefore()
	unsealed.failRecord(oldRecords, context.Canceled)
	if !errors.Is(unsealed.failure, context.Canceled) {
		t.Fatal("control did not reproduce the unsealed cancellation window")
	}
	for _, settled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		stream := &Stream{ctx: ctx, cancel: cancel, settled: settled}
		records := &attachmentRecords{}
		stream.records = records
		stream.sealRetirement()
		if !stream.closing || ctx.Err() == nil {
			t.Fatal("retirement did not deny effects before interruption")
		}
		lateFailure := errors.New("record read interrupted")
		stream.failRecord(records, lateFailure)
		if settled && stream.failure != nil {
			t.Fatal("late record changed settled logical state", stream.failure)
		}
		if !settled && !errors.Is(stream.failure, context.Canceled) {
			t.Fatal("uncompleted retirement lost cancellation", stream.failure)
		}
		if errors.Is(stream.failure, lateFailure) {
			t.Fatal("late record replaced the earlier retirement cause")
		}
	}
}

func TestStreamRetirementSealRetainsOriginalFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	original := errors.New("original authority refused")
	stream := &Stream{ctx: ctx, cancel: cancel, failure: original}
	stream.sealRetirement()
	stream.failRecord(&attachmentRecords{}, context.Canceled)
	if stream.failure != original {
		t.Fatal("retirement replaced original failure", stream.failure)
	}
}

func TestTailReaderDistinguishesPartialServiceRecordFromEmptyTLSTail(t *testing.T) {
	prefix := []byte("ardents-service-connection-v3\x00")
	for size := range prefix {
		read := recordio.NewStreamReader(bytes.NewReader(prefix[:size]))
		_, received, err := read.ReadRecord()
		if received != size || terminalTruncation(err, received) != (size == 0) {
			t.Fatal("actual partial record lost plaintext/physical error", size, received, err)
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

// These ordered controls supply no authority or exact Route CLOSE witness.
func TestTerminalTailReplacementRequiresCurrentControlProofs(t *testing.T) {
	flow := completedTransfer(t)
	stream := &Stream{settled: true, transfer: flow}
	if !stream.terminalTailCompleteLocked() {
		t.Fatal("complete original controls did not permit the separate tail check")
	}
	replacement, err := flow.PrepareReplacement(2, wire.Continuity{})
	if err != nil {
		t.Fatal(err)
	}
	replacement.Commit()
	checkIncomplete := func(boundary string) {
		t.Helper()
		if stream.terminalTailCompleteLocked() {
			t.Fatal("old Application settlement retired incomplete replacement controls", boundary)
		}
	}
	checkIncomplete("replacement")
	finishTerminalRecord(t, flow, false, false)
	checkIncomplete("local Terminal only")
	acceptTerminalRecord(t, flow, wire.Record{Terminal: &wire.Terminal{AttachmentGeneration: 2}})
	finishTerminalRecord(t, flow, true, false)
	checkIncomplete("peer receipt missing")
	acceptTerminalRecord(t, flow, wire.Record{Acknowledgement: &wire.Acknowledgement{AttachmentGeneration: 2, Terminal: true}})
	finishTerminalRecord(t, flow, true, true)
	checkIncomplete("peer confirmation missing")
	acceptTerminalRecord(t, flow, wire.Record{Acknowledgement: &wire.Acknowledgement{AttachmentGeneration: 2, Terminal: true, TerminalConfirmation: true}})
	if !stream.terminalTailCompleteLocked() || !stream.settled {
		t.Fatal("complete replacement controls lost original settlement")
	}
}

func completedTransfer(t *testing.T) *transfer.State {
	t.Helper()
	flow, err := transfer.New(1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.CloseInput(); err != nil {
		t.Fatal(err)
	}
	acceptTerminalRecord(t, flow, wire.Record{Terminal: &wire.Terminal{AttachmentGeneration: 1}})
	finishTerminalRecord(t, flow, false, false)
	finishTerminalRecord(t, flow, true, false)
	acceptTerminalRecord(t, flow, wire.Record{Acknowledgement: &wire.Acknowledgement{AttachmentGeneration: 1, Terminal: true}})
	finishTerminalRecord(t, flow, true, true)
	acceptTerminalRecord(t, flow, wire.Record{Acknowledgement: &wire.Acknowledgement{AttachmentGeneration: 1, Terminal: true, TerminalConfirmation: true}})
	if !flow.DeliveryComplete() {
		t.Fatal("fixture did not complete the terminal exchange")
	}
	return flow
}

func finishTerminalRecord(t *testing.T, flow *transfer.State, acknowledgement, confirmation bool) {
	t.Helper()
	record, ticket, err := flow.BeginRecordWrite()
	if err != nil || ticket == nil {
		t.Fatal("missing original control record", err)
	}
	if acknowledgement {
		if record.Acknowledgement == nil || !record.Acknowledgement.Terminal || record.Acknowledgement.TerminalConfirmation != confirmation {
			t.Fatal("unexpected control acknowledgement", record)
		}
	} else if record.Terminal == nil {
		t.Fatal("expected Terminal", record)
	}
	if flow.DeliveryComplete() {
		t.Fatal("control write became complete before physical result")
	}
	if err := flow.FinishRecordWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
}

func acceptTerminalRecord(t *testing.T, flow *transfer.State, record wire.Record) {
	t.Helper()
	if err := flow.AcceptRecord(record); err != nil {
		t.Fatal("apply terminal control", err)
	}
}
