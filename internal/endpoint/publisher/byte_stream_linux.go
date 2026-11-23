//go:build linux

package publisher

import (
	"context"
	"errors"
	"io"
	"sync"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
	publicationruntime "github.com/dianabuilds/ardents-network/internal/publication/runtime"
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
	contract, err := operation.BindStreamContract(application.ByteStreamDirectionalLimit, application.ByteStreamDirectionalLimit)
	if err != nil {
		return err
	}
	stream, err := publisher.NextStream(ctx, contract)
	if err != nil {
		if publisher.StreamAcquisitionStopped(err) {
			return nil
		}
		return err
	}
	defer func() {
		_ = stream.Close()
		operation.RetainCleanup(stream.JoinedCleanup())
	}()
	exchange, cancel := context.WithTimeout(ctx, application.ReaderTimeout)
	defer cancel()
	phase = "Publisher worker exchange failed"
	if err := operation.ExchangeByteStream(exchange, contract, port, stream); err != nil {
		return err
	}
	phase = "Publisher did not obtain authenticated terminal completion"
	select {
	case outcome, open := <-stream.Done():
		if !open || outcome.Class != application.CleanClose {
			return errors.New("authenticated byte stream did not complete")
		}
	case <-exchange.Done():
		return exchange.Err()
	}
	phase = "Publisher could not join the original stream"
	// The initiator can still be exchanging final authenticated controls.
	// Keep the original Publication and Instance until that attachment joins.
	select {
	case <-stream.Joined():
		if err := stream.JoinedCleanup(); err != nil {
			return err
		}
	case <-exchange.Done():
		return exchange.Err()
	}
	phase = "Publisher could not withdraw the completed stream"
	return publisher.Withdraw(ctx)
}

// OpenByteStream retains the operator's local byte attachment only after the
// Service's ordinary publication acknowledgement.
func (owner *Publisher) OpenByteStream(ctx context.Context) (application.Stream, error) {
	lifetime, cancel := context.WithCancel(ctx)
	in, write := io.Pipe()
	read, out := io.Pipe()
	port := &publishedBytePort{
		ctx: lifetime, cancel: cancel, in: in, out: out,
		done: make(chan application.Outcome, 1), joined: make(chan struct{}),
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
	done   chan application.Outcome
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
		outcome := application.Outcome{Class: application.CleanClose}
		if err != nil {
			outcome = application.Outcome{Class: application.IndeterminateFailure, Reason: publishedByteStreamFailureReason(err)}
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

func (stream *publishedByteStream) Read(body []byte) (int, error)    { return stream.read.Read(body) }
func (stream *publishedByteStream) Write(body []byte) (int, error)   { return stream.write.Write(body) }
func (stream *publishedByteStream) CloseInput() error                { return stream.write.Close() }
func (stream *publishedByteStream) Done() <-chan application.Outcome { return stream.port.done }
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
