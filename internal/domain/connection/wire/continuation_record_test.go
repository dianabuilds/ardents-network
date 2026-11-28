package wire_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

func TestContinuationBoundaryHasExactDirectionalEncoding(t *testing.T) {
	record := wire.Continuation{
		Kind: wire.ProposeBoundary, AttachmentGeneration: 3,
		Boundary: wire.SegmentBoundary{Next: 2, ToPublisher: 0x102030405, ToReader: 0xaabbccddeeff0022, EOF: 2},
	}
	var encoded bytes.Buffer
	if err := wire.Write(&encoded, wire.Record{Continuation: &record}); err != nil {
		t.Fatal(err)
	}
	const payload = "000000000000000300000000000000020000000102030405aabbccddeeff002202"
	if got := hex.EncodeToString(encoded.Bytes()[encoded.Len()-33:]); got != payload {
		t.Fatalf("boundary payload = %s, want %s", got, payload)
	}
	if _, err := wire.ReadStream(bytes.NewReader(encoded.Bytes())); !errors.Is(err, wire.ErrInvalidRecord) {
		t.Fatalf("ordinary stream parser accepted continuation control: %v", err)
	}
	got, err := wire.Read(&encoded)
	if err != nil || got.Continuation == nil || *got.Continuation != record {
		t.Fatalf("decoded boundary = %+v, %v", got.Continuation, err)
	}
}

func TestContinuationRecordsRefuseMalformedPayload(t *testing.T) {
	boundary := wire.SegmentBoundary{Next: 2, ToPublisher: 5, ToReader: 9}
	ticket, logical := [32]byte{3}, [32]byte{4}
	for _, value := range []wire.Continuation{
		{Kind: wire.DrainSegment, AttachmentGeneration: 1},
		{Kind: wire.ProposeBoundary, AttachmentGeneration: 1, Boundary: boundary},
		{Kind: wire.OfferContinuation, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket},
		{Kind: wire.ConfirmBoundary, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket},
		{Kind: wire.AcceptBoundary, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket},
		{Kind: wire.ResumeTransfer, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket, LogicalContext: logical},
		{Kind: wire.AcceptResume, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket, LogicalContext: logical},
	} {
		var valid bytes.Buffer
		if err := wire.Write(&valid, wire.Record{Continuation: &value}); err != nil {
			t.Fatal(err)
		}
		wireBytes := valid.Bytes()
		got, err := wire.Read(bytes.NewReader(wireBytes))
		if err != nil || got.Continuation == nil || *got.Continuation != value {
			t.Fatalf("kind %d = %+v, %v", value.Kind, got.Continuation, err)
		}

		// Extend the announced record body by one byte. The reader cannot
		// treat that surplus as part of any known control.
		extended := append(bytes.Clone(wireBytes), 0)
		const lengthOffset = len("ardents-service-connection-v3\x00")
		extended[lengthOffset+1]++
		if _, err := wire.Read(bytes.NewReader(extended)); !errors.Is(err, wire.ErrInvalidRecord) {
			t.Fatalf("kind %d accepted surplus: %v", value.Kind, err)
		}
	}
	for _, value := range []wire.Continuation{
		{Kind: wire.DrainSegment},
		{Kind: wire.DrainSegment, AttachmentGeneration: 1, Boundary: boundary},
		{Kind: wire.ProposeBoundary, AttachmentGeneration: 1, Boundary: wire.SegmentBoundary{Next: 1}},
		{Kind: wire.ProposeBoundary, AttachmentGeneration: 1, Boundary: wire.SegmentBoundary{Next: 2, EOF: 4}},
		{Kind: wire.ProposeBoundary, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket},
		{Kind: wire.OfferContinuation, AttachmentGeneration: 1, Boundary: boundary},
		{Kind: wire.ResumeTransfer, AttachmentGeneration: 1, Boundary: boundary, Ticket: ticket},
		{Kind: wire.ContinuationKind(14), AttachmentGeneration: 1},
	} {
		var output bytes.Buffer
		if err := wire.Write(&output, wire.Record{Continuation: &value}); err == nil || output.Len() != 0 {
			t.Fatalf("invalid kind %d emitted %d bytes, error %v", value.Kind, output.Len(), err)
		}
	}
}
