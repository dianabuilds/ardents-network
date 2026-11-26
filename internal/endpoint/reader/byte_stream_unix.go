//go:build linux || darwin

package reader

import (
	"context"
	"errors"
	"io"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/connection"
)

type localByteStreamPort struct {
	input  *io.PipeReader
	output *io.PipeWriter
}

func (port localByteStreamPort) Read(body []byte) (int, error)  { return port.input.Read(body) }
func (port localByteStreamPort) Write(body []byte) (int, error) { return port.output.Write(body) }
func (port localByteStreamPort) CloseInput() error              { return port.output.Close() }
func (port localByteStreamPort) Close() error {
	return errors.Join(port.input.Close(), port.output.Close())
}

func newByteStreamResult(original context.Context, check func() error, pending *preparedRead) *resultStream {
	ctx, cancel := context.WithCancel(original)
	request, input := io.Pipe()
	output, response := io.Pipe()
	stream := &resultStream{input: input, output: output, cancel: cancel, joined: make(chan struct{}), done: make(chan application.Outcome, 1)}
	go func() {
		defer close(stream.joined)
		defer cancel()
		local := localByteStreamPort{input: request, output: response}
		op := pending.read.operation
		reason := "byte stream lost local admission"
		err := errors.Join(ctx.Err(), check())
		if err == nil {
			err = op.ExchangeByteStream(ctx, pending.read.contract, local, pending.service)
			if err != nil {
				reason = connection.StreamFailureReason(err)
				if reason == "" {
					reason = document.ByteStreamFailure(err)
				}
			}
		}
		if err == nil {
			reason = "Service Connection did not authenticate terminal completion"
			select {
			case outcome, open := <-pending.service.Done():
				if !open || outcome.Class != application.CleanClose {
					err = errors.New("authenticated byte stream did not complete")
				}
			case <-ctx.Done():
				err = ctx.Err()
			}
		}
		if err == nil {
			reason = "byte-stream joined cleanup failed"
		}
		err = errors.Join(err, pending.close(), ctx.Err(), check())
		stream.err = err
		_ = request.CloseWithError(err)
		_ = response.CloseWithError(err)
		if err == nil {
			stream.done <- application.Outcome{Class: application.CleanClose}
		} else {
			stream.done <- application.Outcome{Class: application.IndeterminateFailure, Reason: reason}
		}
		close(stream.done)
	}()
	return stream
}
