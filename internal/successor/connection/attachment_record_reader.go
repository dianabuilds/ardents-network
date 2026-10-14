package connection

import (
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// One reader actor owns this fixed framing scratch space. Prefetched bytes are
// unaccepted: every decoded record still crosses the Stream's original-owner
// commit barrier. The receive/replay ledger keeps its separate 256 KiB bound.
// No background read, timer, generation transfer or accepting result is stored.
type attachmentRecordReader struct {
	reader     io.Reader
	buffer     [512]byte
	start, end int
	failure    error
}

func (input *attachmentRecordReader) Read(body []byte) (int, error) {
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

func (input *attachmentRecordReader) consume(body []byte) int {
	n := copy(body, input.buffer[input.start:input.end])
	clear(input.buffer[input.start : input.start+n])
	input.start += n
	return n
}

func (input *attachmentRecordReader) readRecord() (wire.StreamRecord, int, error) {
	observed := observedRecordRead{reader: input}
	record, err := wire.ReadStream(&observed)
	if input.failure != nil {
		// ReadFull can hide an error accompanying enough bytes. A complete
		// buffered record must not hide that original transport/owner failure.
		return wire.StreamRecord{}, observed.received, errors.Join(err, input.failure)
	}
	return record, observed.received, err
}

func (input *attachmentRecordReader) discard() {
	clear(input.buffer[:])
	input.start, input.end = 0, 0
	input.reader = nil
}
