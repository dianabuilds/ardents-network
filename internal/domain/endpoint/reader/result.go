package reader

import (
	"bytes"
	"context"
	"errors"
	"io"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

// resultStream owns only local request/result projection. Execution
// owns the worker and sealed result; Source owns authenticated network cleanup.
type resultStream struct {
	input  *io.PipeWriter
	output *io.PipeReader
	cancel context.CancelFunc
	joined chan struct{}
	done   chan application.Outcome
	err    error
}

func newResultStream(original context.Context, check func() error, complete func() ([]byte, error), retire func() error) *resultStream {
	ctx, cancel := context.WithCancel(original)
	request, input := io.Pipe()
	output, response := io.Pipe()
	stream := &resultStream{input: input, output: output, cancel: cancel, joined: make(chan struct{}), done: make(chan application.Outcome, 1)}
	go func() {
		defer close(stream.joined)
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			defer close(interrupted)
			_ = request.CloseWithError(ctx.Err())
			_ = response.CloseWithError(ctx.Err())
			_ = retire()
		})
		var fixed bytes.Buffer
		empty, err := document.NewSnapshot(nil)
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			err = empty.Respond(io.TeeReader(request, &fixed), io.Discard)
		}
		var body []byte
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			body, err = complete()
		}
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			var snapshot *document.Snapshot
			snapshot, err = document.NewSnapshot(body)
			clear(body)
			if err == nil {
				err = snapshot.Respond(bytes.NewReader(fixed.Bytes()), checkedOutput{ctx: ctx, check: check, output: response})
			}
		}
		clear(body)
		err = errors.Join(err, retire(), ctx.Err(), check())
		if !stop() {
			<-interrupted
		}
		stream.err = err
		outcome := application.Outcome{Class: application.CleanClose}
		if err != nil {
			outcome = application.Outcome{Class: application.IndeterminateFailure, Reason: "document read did not complete"}
		}
		_ = request.CloseWithError(err)
		_ = response.CloseWithError(err)
		stream.done <- outcome
		close(stream.done)
		cancel()
	}()
	return stream
}

type checkedOutput struct {
	ctx    context.Context
	check  func() error
	output io.Writer
}

func (output checkedOutput) Write(body []byte) (int, error) {
	if err := errors.Join(output.ctx.Err(), output.check()); err != nil {
		return 0, err
	}
	n, err := output.output.Write(body)
	return n, errors.Join(err, output.ctx.Err(), output.check())
}

func (stream *resultStream) Read(body []byte) (int, error)    { return stream.output.Read(body) }
func (stream *resultStream) Write(body []byte) (int, error)   { return stream.input.Write(body) }
func (stream *resultStream) CloseInput() error                { return stream.input.Close() }
func (stream *resultStream) Done() <-chan application.Outcome { return stream.done }
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
