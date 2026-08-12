package byteio

import "bytes"

// Buffer accepts every caller write while retaining only its fixed byte limit.
type Buffer struct {
	buffer    bytes.Buffer
	remaining int
	overflow  bool
}

// NewBuffer creates one bounded output capture.
func NewBuffer(maximum int) *Buffer {
	if maximum < 0 {
		maximum = 0
	}
	return &Buffer{remaining: maximum}
}

// Write implements io.Writer without applying backpressure to the observed process.
func (writer *Buffer) Write(raw []byte) (int, error) {
	original := len(raw)
	if len(raw) > writer.remaining {
		writer.overflow = true
		raw = raw[:writer.remaining]
	}
	_, _ = writer.buffer.Write(raw)
	writer.remaining -= len(raw)
	return original, nil
}

// Bytes returns a copy of the retained prefix.
func (writer *Buffer) Bytes() []byte { return append([]byte(nil), writer.buffer.Bytes()...) }

// Overflowed reports whether any bytes were discarded.
func (writer *Buffer) Overflowed() bool { return writer.overflow }
