package recordio

import (
	"bufio"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// NewInitialRequestReader bounds prefetch to the complete initial Reader flight.
func NewInitialRequestReader(reader io.Reader) *InitialReader {
	return newInitialReader(reader, wire.InitialRequestBytes)
}

// NewInitialResponseReader bounds prefetch to the complete initial Publisher flight.
func NewInitialResponseReader(reader io.Reader) *InitialReader {
	return newInitialReader(reader, wire.InitialResponseBytes)
}

// newInitialReader coalesces only one fixed authentication flight. Its
// limit prevents buffering any following Application record that must remain
// with the authenticated Attachment. Record parsing and owner checks still
// run separately, and fragmented transport retains ordinary Read semantics.
func newInitialReader(reader io.Reader, size int) *InitialReader {
	input := &initialFlightInput{reader: reader}
	return &InitialReader{input: input, buffered: bufio.NewReaderSize(io.LimitReader(input, int64(size)), size)}
}

// InitialReader consumes only a fixed initial request or response flight.
type InitialReader struct {
	input    *initialFlightInput
	buffered *bufio.Reader
}

// ReadRecord reads the next record and retains any accompanying transport error.
func (flight *InitialReader) ReadRecord() (wire.Record, error) {
	record, err := wire.Read(flight.buffered)
	if flight.input.failure != nil {
		return wire.Record{}, errors.Join(err, flight.input.failure)
	}
	return record, err
}

// Buffered parsing must not hide a failed original read that returned bytes.
// Both records remain private and refuse even if those bytes form a full flight.
type initialFlightInput struct {
	reader  io.Reader
	failure error
}

func (input *initialFlightInput) Read(body []byte) (int, error) {
	n, err := input.reader.Read(body)
	if err != nil && input.failure == nil {
		input.failure = err
	}
	return n, err
}
