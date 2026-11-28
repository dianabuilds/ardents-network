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

func TestSegmentBackpressureRetainsOneUnwrittenRecord(t *testing.T) {
	state, err := transfer.New(1, 4*wire.MaximumDataBytes, 4*wire.MaximumDataBytes)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, wire.MaximumDataBytes)
	if err := state.CheckSegmentWrite(data); err != nil {
		t.Fatal(err)
	}
	if err := state.Write(data); err != nil {
		t.Fatal(err)
	}
	_, ticket, err := state.BeginRecordWrite()
	if err != nil || ticket == nil {
		t.Fatal("original data record unavailable", err)
	}
	if err := state.CheckSegmentWrite(data[:1]); !errors.Is(err, transfer.ErrBackpressure) {
		t.Fatal("unsent data escaped the physical write bound", err)
	}
	if err := state.FinishRecordWrite(ticket, nil); err != nil {
		t.Fatal(err)
	}
	if offsets := state.ReplayOffsets(); offsets.SendBase != 0 || offsets.SendEnd != wire.MaximumDataBytes {
		t.Fatalf("physical write advanced the authenticated replay floor: %+v", offsets)
	}
	if err := state.CheckSegmentWrite(data); err != nil {
		t.Fatal("write waited for a peer ACK with physical capacity available", err)
	}
	if end := state.FreezeSegment(); end != wire.MaximumDataBytes {
		t.Fatalf("local backpressure changed the agreed cut: %d", end)
	}
	if err := state.CheckSegmentWrite(data[:1]); !errors.Is(err, transfer.ErrSegmentDraining) {
		t.Fatal("frozen segment admitted further bytes", err)
	}
}
