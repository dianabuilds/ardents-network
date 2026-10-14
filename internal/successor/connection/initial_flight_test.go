package connection

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

type initialFlightTransport struct {
	*bytes.Reader
	reads, chunk int
	failure      error
}

func (r *initialFlightTransport) Read(body []byte) (int, error) {
	r.reads++
	if r.chunk > 0 && len(body) > r.chunk {
		body = body[:r.chunk]
	}
	n, err := r.Reader.Read(body)
	return n, errors.Join(err, r.failure)
}

// These are grammar-only flights, not authenticated Instance/Connection facts.
// The real production reader must not consume the following stream's bytes.
func TestInitialFlightReaderPreservesFollowingApplicationBytes(t *testing.T) {
	for _, response := range []bool{false, true} {
		first := wire.Record{Challenge: &wire.Challenge{Network: [32]byte{1}, Target: [32]byte{2},
			InstanceGeneration: 1, Context: [32]byte{3}, Nonce: [32]byte{4}}}
		role, size, independentSize := wire.RoleInitiator, wire.InitialRequestBytes, 443
		if response {
			first = wire.Record{Proof: &wire.Proof{ChallengeDigest: [32]byte{1}, Signature: [64]byte{2}}}
			role, size, independentSize = wire.RolePublisher, wire.InitialResponseBytes, 403
		}
		continuity, err := wire.NewContinuity([32]byte{1}, role, 1, 0, 0, 0, [32]byte{2}, [32]byte{3})
		if err != nil {
			t.Fatal(err)
		}
		var flight bytes.Buffer
		if err := wire.Write(&flight, first); err != nil {
			t.Fatal(err)
		}
		if err := wire.Write(&flight, wire.Record{Continuity: &continuity}); err != nil {
			t.Fatal(err)
		}
		if size != independentSize || flight.Len() != independentSize {
			t.Fatal("fixed v3 flight length changed", response, size, flight.Len())
		}
		for _, chunk := range []int{0, 7} {
			tail := []byte("following Application record")
			transport := &initialFlightTransport{Reader: bytes.NewReader(append(append([]byte(nil), flight.Bytes()...), tail...)), chunk: chunk}
			reader := initialFlightReader(transport, size)
			got, err := reader.readRecord()
			if err != nil || response && got.Proof == nil || !response && got.Challenge == nil {
				t.Fatal("first record differs", err)
			}
			got, err = reader.readRecord()
			if err != nil || got.Continuity == nil || *got.Continuity != continuity {
				t.Fatal("second record differs", err)
			}
			if chunk == 0 && transport.reads != 1 {
				t.Fatal("coalesced flight repeated transport observation", transport.reads)
			}
			if transport.Len() != len(tail) {
				t.Fatal("authentication consumed following Application bytes")
			}
			if _, err := reader.buffered.ReadByte(); !errors.Is(err, io.EOF) || transport.Len() != len(tail) {
				t.Fatal("flight limit allowed later bytes", err)
			}
		}
		truncated := initialFlightReader(bytes.NewReader(flight.Bytes()[:flight.Len()-1]), size)
		if _, err := truncated.readRecord(); err != nil {
			t.Fatal(err)
		}
		if _, err := truncated.readRecord(); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal("truncated coalesced flight was not refused", err)
		}
		failure := errors.New("original transport refused with a complete flight")
		failed := initialFlightReader(&initialFlightTransport{Reader: bytes.NewReader(flight.Bytes()), failure: failure}, size)
		if record, err := failed.readRecord(); !errors.Is(err, failure) || record.Challenge != nil || record.Proof != nil {
			t.Fatal("buffering hid the failed original read", err)
		}
	}
}
