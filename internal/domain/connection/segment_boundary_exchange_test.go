package connection

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/transfer"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func segmentFlow(t *testing.T) *transfer.State {
	t.Helper()
	flow, err := transfer.New(1, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	return flow
}

func segmentWrite(t *testing.T, sender *segmentBoundaryExchange, flow *transfer.State, kind wire.ContinuationKind) *segmentControlWrite {
	t.Helper()
	write, err := sender.beginWrite(flow)
	if err != nil || write == nil || write.record.Kind != kind {
		t.Fatalf("expected control %d: %+v, %v", kind, write, err)
	}
	return write
}

func segmentExchangeFrame(t *testing.T, sender *segmentBoundaryExchange, sending *transfer.State, peer *segmentBoundaryExchange, receiving *transfer.State, kind wire.ContinuationKind) {
	t.Helper()
	write := segmentWrite(t, sender, sending, kind)
	if err := peer.receive(receiving, write.record); err != nil {
		t.Fatal(err)
	}
	if err := sender.finishWrite(write, sending.Generation(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestSegmentBoundaryRequiresPeerFinalOffsetBeforeProposal(t *testing.T) {
	flow := segmentFlow(t)
	reader, err := newSegmentBoundaryExchange(wire.RoleInitiator, wire.SegmentBoundary{Next: 1}, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	reader.freeze(flow)
	write := segmentWrite(t, reader, flow, wire.DrainSegment)
	if err := reader.finishWrite(write, 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := reader.receive(flow, wire.Continuation{Kind: wire.DrainSegment, AttachmentGeneration: 1, DrainOffset: 3}); err != nil {
		t.Fatal(err)
	}
	if write, err := reader.beginWrite(flow); write != nil || err != nil {
		t.Fatalf("unread peer range advanced the boundary: %+v, %v", write, err)
	}
	if reader.acceptsData(wire.Data{Offset: 2, Payload: []byte{1, 2}}) {
		t.Fatal("peer extended its declared cut")
	}
	if err := flow.AcceptRecord(wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: []byte{1, 2, 3}}}); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 3)
	if n, err := flow.Read(data); n != 3 || err != nil {
		t.Fatalf("read original queued data: %d, %v", n, err)
	}
	_, ack, err := flow.BeginRecordWrite()
	if err != nil || ack == nil {
		t.Fatalf("original receive acknowledgement: %v", err)
	}
	if err := flow.FinishRecordWrite(ack, nil); err != nil {
		t.Fatal(err)
	}
	proposal := segmentWrite(t, reader, flow, wire.ProposeBoundary)
	if proposal.record.Boundary != (wire.SegmentBoundary{Next: 2, ToReader: 3}) {
		t.Fatalf("proposal = %+v", proposal.record.Boundary)
	}
}

func TestSegmentBoundaryCompletesOnlyAfterCurrentPhysicalConfirmation(t *testing.T) {
	base := wire.SegmentBoundary{Next: 2, ToPublisher: 6, ToReader: 11, EOF: 1}
	reader, err := newSegmentBoundaryExchange(wire.RoleInitiator, base, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := newSegmentBoundaryExchange(wire.RolePublisher, base, [32]byte{3})
	if err != nil {
		t.Fatal(err)
	}
	readFlow, publishFlow := segmentFlow(t), segmentFlow(t)
	reader.freeze(readFlow)
	segmentExchangeFrame(t, reader, readFlow, publisher, publishFlow, wire.DrainSegment)
	segmentExchangeFrame(t, publisher, publishFlow, reader, readFlow, wire.DrainSegment)
	segmentExchangeFrame(t, reader, readFlow, publisher, publishFlow, wire.ProposeBoundary)
	segmentExchangeFrame(t, publisher, publishFlow, reader, readFlow, wire.OfferContinuation)
	confirm := segmentWrite(t, reader, readFlow, wire.ConfirmBoundary)
	if err := publisher.receive(publishFlow, confirm.record); err != nil {
		t.Fatal(err)
	}
	answer := segmentWrite(t, publisher, publishFlow, wire.AcceptBoundary)
	if err := reader.receive(readFlow, answer.record); err != nil {
		t.Fatal(err)
	}
	if reader.complete(1) || publisher.complete(1) {
		t.Fatal("boundary completed before original write returned")
	}
	if err := reader.finishWrite(confirm, 1, nil); err != nil {
		t.Fatal(err)
	}
	lost := errors.New("original attachment failed after accepting write")
	if err := publisher.finishWrite(answer, 1, lost); !errors.Is(err, lost) || publisher.complete(1) {
		t.Fatal("failed original write counted as successful confirmation", err)
	}
	retry := segmentWrite(t, publisher, publishFlow, wire.AcceptBoundary)
	if err := publisher.finishWrite(retry, 1, nil); err != nil {
		t.Fatal(err)
	}
	if !reader.complete(1) || !publisher.complete(1) || reader.complete(2) || publisher.complete(2) {
		t.Fatal("confirmation was lost or rebound to a different Attachment")
	}
	want := wire.SegmentBoundary{Next: 3, ToPublisher: 6, ToReader: 11, EOF: 1}
	if retry.record.Boundary != want || retry.record != answer.record {
		t.Fatalf("replayed result = %+v; want boundary %+v", retry.record, want)
	}
}
