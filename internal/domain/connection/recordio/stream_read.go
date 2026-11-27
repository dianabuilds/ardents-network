package recordio

import (
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// One reader actor owns this fixed framing scratch space. Prefetched bytes are
// unaccepted: every decoded record still crosses the Stream's original-owner
// commit barrier. The receive/replay ledger keeps its separate 256 KiB bound.
// No background read, timer, generation transfer or accepting result is stored.
type StreamReader struct {
	reader     io.Reader
	buffer     [512]byte
	start, end int
	failure    error
}

// NewStreamReader binds one record reader to the actor's physical input.
func NewStreamReader(reader io.Reader) *StreamReader {
	return &StreamReader{reader: reader}
}

// Read supplies plaintext to the wire decoder.
func (input *StreamReader) Read(body []byte) (int, error) {
	if input.failure != nil {
		return 0, input.failure
	}
	if len(body) == 0 {
		return 0, nil
	}
	if input.start < input.end {
		return input.consume(body), nil
	}
	if len(body) >= len(input.buffer) {
		n, err := input.reader.Read(body)
		input.failure = err
		return n, err
	}
	n, err := input.reader.Read(input.buffer[:])
	input.start, input.end, input.failure = 0, n, err
	return input.consume(body), err
}

func (input *StreamReader) consume(body []byte) int {
	n := copy(body, input.buffer[input.start:input.end])
	clear(input.buffer[input.start : input.start+n])
	input.start += n
	return n
}

// ReadRecord reads one Stream record and reports the plaintext bytes consumed.
func (input *StreamReader) ReadRecord() (wire.StreamRecord, int, error) {
	observed := observedRecordRead{reader: input}
	record, err := wire.ReadStream(&observed)
	if input.failure != nil {
		// ReadFull can hide an error accompanying enough bytes. A complete
		// buffered record must not hide that original transport/owner failure.
		return wire.StreamRecord{}, observed.received, errors.Join(err, input.failure)
	}
	return record, observed.received, err
}

// Discard clears the reader scratch space and drops its transport reference.
func (input *StreamReader) Discard() {
	clear(input.buffer[:])
	input.start, input.end = 0, 0
	input.reader = nil
}

// Record parsing must observe no additional plaintext for a TLS-tail
// truncation exception. A partially received Service record is not an empty
// TLS tail, even when its final physical error is EOF.
type observedRecordRead struct {
	reader   io.Reader
	received int
}

func (read *observedRecordRead) Read(body []byte) (int, error) {
	n, err := read.reader.Read(body)
	read.received += n
	return n, err
}
