package reader

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// resultStream owns the local duplex pipes and joined terminal result.
// Execution owns the worker; Source owns authenticated network cleanup.
type resultStream struct {
	input  *io.PipeWriter
	output *io.PipeReader
	cancel context.CancelFunc
	joined chan struct{}
	done   chan endpointstream.Outcome
	err    error
}

func (stream *resultStream) Read(body []byte) (int, error)       { return stream.output.Read(body) }
func (stream *resultStream) Write(body []byte) (int, error)      { return stream.input.Write(body) }
func (stream *resultStream) CloseInput() error                   { return stream.input.Close() }
func (stream *resultStream) Done() <-chan endpointstream.Outcome { return stream.done }
func (stream *resultStream) Close() error {
	select {
	case <-stream.joined:
		return stream.err
	default:
		stream.cancel()
		_ = stream.input.CloseWithError(context.Canceled)
		_ = stream.output.CloseWithError(context.Canceled)
		<-stream.joined
		return stream.err
	}
}
