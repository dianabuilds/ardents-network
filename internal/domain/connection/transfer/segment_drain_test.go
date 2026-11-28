package transfer_test

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestDrainWaitsForPhysicalAcknowledgementsAndLocalDelivery(t *testing.T) {
	state, err := transfer.New(1, 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Write([]byte("send")); err != nil {
		t.Fatal(err)
	}
	if err := state.AcceptRecord(wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: []byte("receive")}}); err != nil {
		t.Fatal(err)
	}

	expectPending := func(label string) {
		t.Helper()
		if got, ok := state.DrainedSegment(); ok {
			t.Fatalf("%s: unfinished segment exposed positions %+v", label, got)
		}
	}
	expectPending("before freeze")
	state.FreezeSegment()
	if err := state.CheckWrite([]byte("x")); !errors.Is(err, transfer.ErrSegmentDraining) {
		t.Fatalf("write preview = %v", err)
	}
	if err := state.Write([]byte("x")); !errors.Is(err, transfer.ErrSegmentDraining) {
		t.Fatalf("write during drain = %v", err)
	}
	if err := state.CloseInput(); !errors.Is(err, transfer.ErrSegmentDraining) || state.InputClosed() {
		t.Fatal("segment drain closed Application input", err)
	}
	expectPending("pending acknowledgement and data")

	sendRecord := func() wire.Record {
		t.Helper()
		record, ticket, err := state.BeginRecordWrite()
		if err != nil || ticket == nil {
			t.Fatalf("next record: %+v, %v", record, err)
		}
		if err := state.FinishRecordWrite(ticket, nil); err != nil {
			t.Fatal(err)
		}
		return record
	}
	if record := sendRecord(); record.Acknowledgement == nil || record.Acknowledgement.Offset != 7 {
		t.Fatalf("first record did not acknowledge original receive: %+v", record)
	}
	if record := sendRecord(); record.Data == nil || string(record.Data.Payload) != "send" {
		t.Fatalf("record did not drain original send: %+v", record)
	}
	expectPending("send written without peer acknowledgement")
	if err := state.AcceptRecord(wire.Record{Acknowledgement: &wire.Acknowledgement{AttachmentGeneration: 1, Offset: 4}}); err != nil {
		t.Fatal(err)
	}
	expectPending("receive not delivered")
	buf := make([]byte, 7)
	if n, err := state.Read(buf); err != nil || n != 7 || string(buf) != "receive" {
		t.Fatalf("original delivered bytes = %q, %v", buf[:n], err)
	}
	if n, err := state.Read(buf); n != 0 || err != nil {
		t.Fatalf("empty drained segment exposed data or EOF: %d, %v", n, err)
	}
	want := transfer.SegmentPositions{Sent: 4, Received: 7}
	if got, ok := state.DrainedSegment(); !ok || got != want {
		t.Fatalf("drained segment = %+v, %t; want %+v", got, ok, want)
	}
}

func TestDrainPreservesOnlyConfirmedDirectionalEOF(t *testing.T) {
	state, err := transfer.New(1, 32, 32)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.CloseInput(); err != nil {
		t.Fatal(err)
	}
	state.FreezeSegment()
	send := func() wire.Record {
		t.Helper()
		record, ticket, err := state.BeginRecordWrite()
		if err != nil || ticket == nil {
			t.Fatalf("next record: %+v, %v", record, err)
		}
		if err := state.FinishRecordWrite(ticket, nil); err != nil {
			t.Fatal(err)
		}
		return record
	}
	if record := send(); record.Terminal == nil {
		t.Fatal("original EOF did not produce Terminal")
	}
	if _, ok := state.DrainedSegment(); ok {
		t.Fatal("unacknowledged EOF passed the drain")
	}
	if err := state.AcceptRecord(wire.Record{Acknowledgement: &wire.Acknowledgement{
		AttachmentGeneration: 1, Terminal: true,
	}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := state.DrainedSegment(); ok {
		t.Fatal("EOF passed before physical confirmation was written")
	}
	if record := send(); record.Acknowledgement == nil || !record.Acknowledgement.TerminalConfirmation {
		t.Fatal("original EOF confirmation was not written")
	}
	if got, ok := state.DrainedSegment(); !ok || got != (transfer.SegmentPositions{SentEOF: true}) {
		t.Fatalf("drain results = %+v, %t; expected only the outgoing EOF", got, ok)
	}
}
