//go:build linux || darwin

package publisher

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/domain/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/domain/publication/runtime"
)

func serveByteStream(ctx context.Context, publisher *publicationruntime.Publisher, operation *executionruntime.Operation, port document.ByteStreamPort) (result error) {
	phase := "Publisher stream admission failed"
	defer func() {
		if result != nil {
			result = bytePublicationFailure{phase: phase, cause: result}
		}
	}()
	defer port.Close()
	if local, ok := port.(*publishedBytePort); ok {
		lifetime, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(local.ctx, cancel)
		defer stop()
		ctx = lifetime
	}
	contract, err := operation.BindStreamContract(endpointstream.ByteStreamSegmentDirectionalLimit, endpointstream.ByteStreamSegmentDirectionalLimit)
	if err != nil {
		return err
	}
	open := func(ctx context.Context, previous *connection.Stream) (*connection.Stream, func() error, error) {
		segment, err := publisher.NextSegment(ctx, contract, previous)
		if err != nil {
			return nil, nil, err
		}
		release := func() error {
			err := segment.Close()
			operation.RetainCleanup(segment.JoinedCleanup())
			return err
		}
		return segment, release, nil
	}
	first, release, err := open(ctx, nil)
	if err != nil {
		if publisher.StreamAcquisitionStopped(err) {
			return nil
		}
		return err
	}
	stream, err := connection.OpenTransfer(ctx, first, release, open)
	if err != nil {
		return errors.Join(err, release())
	}
	defer stream.Close()
	phase = "Publisher worker exchange failed"
	if err := operation.ExchangeByteStream(ctx, contract, port, stream); err != nil {
		return err
	}
	phase = "Publisher did not obtain authenticated terminal completion"
	select {
	case outcome, open := <-stream.Done():
		if !open || outcome.Class != endpointstream.CleanClose {
			return errors.New("authenticated byte stream did not complete")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	phase = "Publisher could not withdraw the completed stream"
	return publisher.Withdraw(ctx)
}

// OpenByteStream retains the operator's local byte attachment only after the
// Service's ordinary publication acknowledgement.
func (owner *Publisher) OpenByteStream(ctx context.Context) (endpointstream.Stream, error) {
	lifetime, cancel := context.WithCancel(ctx)
	in, write := io.Pipe()
	read, out := io.Pipe()
	port := &publishedBytePort{
		ctx: lifetime, cancel: cancel, in: in, out: out,
		done: make(chan endpointstream.Outcome, 1), joined: make(chan struct{}),
	}
	stream := &publishedByteStream{read: read, write: write, port: port}
	if err := owner.PublishByteStream(ctx, port); err != nil {
		port.finish(err)
		_ = stream.Close()
		return nil, err
	}
	return stream, nil
}

type publishedBytePort struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     *io.PipeReader
	out    *io.PipeWriter
	done   chan endpointstream.Outcome
	joined chan struct{}
	once   sync.Once
	err    error
}

func (port *publishedBytePort) Read(body []byte) (int, error)  { return port.in.Read(body) }
func (port *publishedBytePort) Write(body []byte) (int, error) { return port.out.Write(body) }
func (port *publishedBytePort) CloseInput() error              { return port.out.Close() }
func (port *publishedBytePort) Close() error {
	return errors.Join(port.in.Close(), port.out.Close())
}
func (port *publishedBytePort) finish(err error) {
	port.once.Do(func() {
		defer close(port.joined)
		port.err = err
		_ = port.Close()
		outcome := endpointstream.Outcome{Class: endpointstream.CleanClose}
		if err != nil {
			outcome = endpointstream.Outcome{Class: endpointstream.IndeterminateFailure, Reason: publishedByteStreamFailureReason(err)}
		}
		port.done <- outcome
		close(port.done)
		port.cancel()
	})
}

type publishedByteStream struct {
	read  *io.PipeReader
	write *io.PipeWriter
	port  *publishedBytePort
}

func (stream *publishedByteStream) Read(body []byte) (int, error)       { return stream.read.Read(body) }
func (stream *publishedByteStream) Write(body []byte) (int, error)      { return stream.write.Write(body) }
func (stream *publishedByteStream) CloseInput() error                   { return stream.write.Close() }
func (stream *publishedByteStream) Done() <-chan endpointstream.Outcome { return stream.port.done }
func (stream *publishedByteStream) Close() error {
	stream.port.cancel()
	_ = stream.read.Close()
	_ = stream.write.Close()
	<-stream.port.joined
	return stream.port.err
}

type bytePublicationFailure struct {
	phase string
	cause error
}

func (failure bytePublicationFailure) Error() string { return failure.phase }
func (failure bytePublicationFailure) Unwrap() error { return failure.cause }

func publishedByteStreamFailureReason(err error) string {
	if failure, ok := errors.AsType[bytePublicationFailure](err); ok {
		if failure.phase == "Publisher worker exchange failed" {
			return document.ByteStreamFailure(failure.cause)
		}
		return failure.phase
	}
	return "Publisher byte-stream cleanup failed"
}
