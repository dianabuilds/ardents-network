//go:build linux

package textdocument_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

type snapshotConnectionOwner struct {
	snapshot *textdocument.Snapshot
	opens    atomic.Uint32
	bytes    atomic.Uint32
}

func (owner *snapshotConnectionOwner) Open(_ context.Context, request connection.Request) (connection.Stream, error) {
	if request.Destination != connection.TargetLink || request.Value != "fixture-link" {
		return nil, errors.New("fixture destination refused")
	}
	owner.opens.Add(1)
	input, requestSink := io.Pipe()
	response, output := io.Pipe()
	stream := &snapshotConnection{request: requestSink, response: response, input: input, output: output, finished: make(chan struct{}), done: make(chan connection.Outcome, 1), bytes: &owner.bytes}
	go func() {
		defer close(stream.finished)
		err := owner.snapshot.Respond(input, output)
		_ = output.CloseWithError(err)
		class := connection.CleanClose
		if err != nil {
			class = connection.IndeterminateFailure
		}
		stream.done <- connection.Outcome{Class: class}
		close(stream.done)
	}()
	return stream, nil
}

type snapshotConnection struct {
	request   *io.PipeWriter
	response  *io.PipeReader
	input     *io.PipeReader
	output    *io.PipeWriter
	finished  chan struct{}
	done      chan connection.Outcome
	closeOnce sync.Once
	bytes     *atomic.Uint32
}

func (stream *snapshotConnection) Read(body []byte) (int, error) { return stream.response.Read(body) }
func (stream *snapshotConnection) Write(body []byte) (int, error) {
	n, err := stream.request.Write(body)
	stream.bytes.Add(uint32(n))
	return n, err
}
func (stream *snapshotConnection) CloseInput() error               { return stream.request.Close() }
func (stream *snapshotConnection) Done() <-chan connection.Outcome { return stream.done }
func (stream *snapshotConnection) Close() error {
	stream.closeOnce.Do(func() {
		_ = stream.request.Close()
		_ = stream.input.Close()
		_ = stream.response.Close()
		_ = stream.output.Close()
		<-stream.finished
	})
	return nil
}
