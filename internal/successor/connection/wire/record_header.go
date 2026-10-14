package wire

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

// Read-ahead stops at the fixed envelope header. The original reader retains
// the whole body and following records; body allocation follows validation.
// A partial read can already reveal an invalid prefix, without waiting for
// the length bytes that a malicious peer might never send.
func readRecordHeader(reader io.Reader) (int, error) {
	input := &recordHeaderInput{reader: reader}
	bound := len(connectionPrefix) + 2
	header := bufio.NewReaderSize(io.LimitReader(input, int64(bound)), bound)
	prefix := make([]byte, len(connectionPrefix))
	if _, err := io.ReadFull(header, prefix); err != nil {
		return 0, errors.Join(err, input.failure)
	}
	if string(prefix) != connectionPrefix {
		return 0, errors.Join(ErrInvalidRecord, errors.New("native connection record prefix is invalid"), input.failure)
	}
	var length [2]byte
	if _, err := io.ReadFull(header, length[:]); err != nil {
		return 0, errors.Join(err, input.failure)
	}
	size := int(binary.BigEndian.Uint16(length[:]))
	if size < commonHeaderSize || size > maximumBodySize {
		return 0, errors.Join(ErrInvalidRecord, errors.New("native connection record body is outside its bound"), input.failure)
	}
	return size, input.failure
}

// Buffering a complete header must not hide a failed original read and then
// resume body acquisition through the underlying reader as if it succeeded.
type recordHeaderInput struct {
	reader  io.Reader
	failure error
}

func (input *recordHeaderInput) Read(body []byte) (int, error) {
	n, err := input.reader.Read(body)
	if err != nil && input.failure == nil {
		input.failure = err
	}
	return n, err
}
