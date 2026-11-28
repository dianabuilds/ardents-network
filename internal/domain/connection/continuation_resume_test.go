package connection

import (
	"bytes"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

type resumeFlight struct {
	input, output bytes.Buffer
}

func (flight *resumeFlight) Read(body []byte) (int, error)  { return flight.input.Read(body) }
func (flight *resumeFlight) Write(body []byte) (int, error) { return flight.output.Write(body) }

func TestReaderResumeRequiresExactAuthenticatedAnswer(t *testing.T) {
	request := wire.Continuation{
		Kind: wire.ResumeTransfer, AttachmentGeneration: initialAttachmentGeneration,
		Boundary: wire.SegmentBoundary{Next: 2, ToPublisher: 7, ToReader: 11, EOF: 2},
		Ticket:   [32]byte{3}, LogicalContext: [32]byte{4},
	}
	answer := request
	answer.Kind = wire.AcceptResume
	for _, test := range []struct {
		name   string
		record wire.Record
		accept bool
	}{
		{"confirmed", wire.Record{Continuation: &answer}, true},
		{"wrong phase", wire.Record{Continuation: &request}, false},
		{"early payload", wire.Record{Data: &wire.Data{AttachmentGeneration: 1, Payload: []byte{42}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var flight resumeFlight
			if err := wire.Write(&flight.input, test.record); err != nil {
				t.Fatal(err)
			}
			err := exchangeReaderResume(&flight, request)
			if test.accept && err != nil || !test.accept && !errors.Is(err, errContinuationRefused) {
				t.Fatalf("Resume result = %v", err)
			}
			sent, err := readResumeRecord(&flight.output, wire.ResumeTransfer)
			if err != nil || sent != request || flight.output.Len() != 0 {
				t.Fatalf("Resume did not write exactly one original request: %+v, %v", sent, err)
			}
		})
	}

	wrong := answer
	wrong.Boundary.EOF = 0
	var flight resumeFlight
	if err := wire.Write(&flight.input, wire.Record{Continuation: &wrong}); err != nil {
		t.Fatal(err)
	}
	if err := exchangeReaderResume(&flight, request); !errors.Is(err, errContinuationRefused) {
		t.Fatalf("changed directional EOF accepted: %v", err)
	}
}

func TestResumedSegmentCannotReopenAnEndedDirection(t *testing.T) {
	base := wire.SegmentBoundary{Next: 2, ToPublisher: 7, ToReader: 11, EOF: 2}
	reader, err := newSegmentBoundaryExchange(wire.RoleInitiator, base, [32]byte{})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := newSegmentBoundaryExchange(wire.RolePublisher, base, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	data := wire.Data{AttachmentGeneration: initialAttachmentGeneration, Payload: []byte{42}}
	if reader.localInputEnded() || !publisher.localInputEnded() ||
		reader.acceptsData(data) || !publisher.acceptsData(data) {
		t.Fatal("continued segment changed the directional EOF state")
	}
}
