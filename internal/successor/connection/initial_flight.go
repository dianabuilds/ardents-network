package connection

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// initialFlightReader coalesces only one fixed authentication flight. Its
// limit prevents buffering any following Application record that must remain
// with the authenticated Attachment. Record parsing and owner checks still
// run separately, and fragmented transport retains ordinary Read semantics.
func initialFlightReader(reader io.Reader, size int) *initialFlight {
	input := &initialFlightInput{reader: reader}
	return &initialFlight{input: input, buffered: bufio.NewReaderSize(io.LimitReader(input, int64(size)), size)}
}

type initialFlight struct {
	input    *initialFlightInput
	buffered *bufio.Reader
}

func (flight *initialFlight) readRecord() (wire.Record, error) {
	record, err := wire.Read(flight.buffered)
	if flight.input.failure != nil {
		return wire.Record{}, errors.Join(err, flight.input.failure)
	}
	return record, err
}

// The setup caller can expire before its cancellation callback interrupts the
// physical Attachment. Retain that original cause after the read has joined;
// neither buffered complete bytes nor physical cancellation extend setup.
func (flight *initialFlight) readSetupRecord(ctx context.Context) (wire.Record, error) {
	if ctx == nil {
		return wire.Record{}, errIntegrity
	}
	if err := ctx.Err(); err != nil {
		return wire.Record{}, err
	}
	record, err := flight.readRecord()
	if errors.Is(err, wire.ErrInvalidRecord) {
		err = errors.Join(errIntegrity, err)
	}
	if err = errors.Join(err, ctx.Err()); err != nil {
		return wire.Record{}, err
	}
	return record, nil
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
