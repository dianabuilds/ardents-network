package endpoint

import (
	"errors"
	"io"
	"sync"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

// applicationHalfClose is one end of an in-process, bidirectional Application
// stream. Closing its input leaves its output readable; Close aborts both
// directions. It is the Endpoint presentation used between the local
// Application Interface and the native Service Connection.
type applicationHalfClose struct {
	reader *io.PipeReader
	writer *io.PipeWriter

	inputOnce sync.Once
	closeOnce sync.Once
	inputErr  error
	closeErr  error
}

var _ nativeconnection.Application = (*applicationHalfClose)(nil)

func newApplicationHalfClosePair() (*applicationHalfClose, *applicationHalfClose) {
	leftReader, rightWriter := io.Pipe()
	rightReader, leftWriter := io.Pipe()
	return &applicationHalfClose{reader: leftReader, writer: leftWriter},
		&applicationHalfClose{reader: rightReader, writer: rightWriter}
}

func (stream *applicationHalfClose) Read(destination []byte) (int, error) {
	if stream == nil || stream.reader == nil {
		return 0, io.ErrClosedPipe
	}
	return stream.reader.Read(destination)
}

func (stream *applicationHalfClose) Write(source []byte) (int, error) {
	if stream == nil || stream.writer == nil {
		return 0, io.ErrClosedPipe
	}
	return stream.writer.Write(source)
}

// CloseInput makes the opposite reader observe EOF without affecting this
// stream's Read direction. It is safe to repeat and rejects later writes.
func (stream *applicationHalfClose) CloseInput() error {
	if stream == nil {
		return io.ErrClosedPipe
	}
	stream.inputOnce.Do(func() { stream.inputErr = stream.writer.Close() })
	return stream.inputErr
}

// Close aborts both directions and wakes any blocked counterpart operation.
func (stream *applicationHalfClose) Close() error {
	if stream == nil {
		return nil
	}
	stream.closeOnce.Do(func() {
		stream.closeErr = errors.Join(stream.CloseInput(), stream.reader.Close())
	})
	return stream.closeErr
}
