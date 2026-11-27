//go:build linux

// This fixture serves only the Linux AAI3 listener control in
// reader_connection_aai3_linux_test.go; the portable reader controls use their
// own admitted fixture streams.
package document_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

type snapshotConnectionOwner struct {
	snapshot []byte
	opens    atomic.Uint32
	bytes    atomic.Uint32
}

func (owner *snapshotConnectionOwner) Open(_ context.Context, request endpointstream.Request) (endpointstream.Stream, error) {
	if request.Destination != endpointstream.TargetLink || request.Value != "fixture-link" {
		return nil, errors.New("fixture destination refused")
	}
	owner.opens.Add(1)
	input, requestSink := io.Pipe()
	response, output := io.Pipe()
	stream := &snapshotConnection{request: requestSink, response: response, input: input, output: output, finished: make(chan struct{}), done: make(chan endpointstream.Outcome, 1), bytes: &owner.bytes}
	go func() {
		defer close(stream.finished)
		var probe [1]byte
		_, err := io.ReadFull(input, probe[:])
		if err == io.EOF {
			_, err = io.Copy(output, bytes.NewReader(owner.snapshot))
		} else if err == nil {
			err = errors.New("unexpected file request payload")
		}
		_ = output.CloseWithError(err)
		class := endpointstream.CleanClose
		if err != nil {
			class = endpointstream.IndeterminateFailure
		}
		stream.done <- endpointstream.Outcome{Class: class}
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
	done      chan endpointstream.Outcome
	closeOnce sync.Once
	bytes     *atomic.Uint32
}

func (stream *snapshotConnection) Read(body []byte) (int, error) { return stream.response.Read(body) }
func (stream *snapshotConnection) Write(body []byte) (int, error) {
	n, err := stream.request.Write(body)
	stream.bytes.Add(uint32(n))
	return n, err
}
func (stream *snapshotConnection) CloseInput() error                   { return stream.request.Close() }
func (stream *snapshotConnection) Done() <-chan endpointstream.Outcome { return stream.done }
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
