package client

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func TestApplicationStreamCopiesBytesAndRequiresAuthenticatedCompletion(t *testing.T) {
	for _, outcome := range []endpointstream.OutcomeClass{endpointstream.CleanClose, endpointstream.IndeterminateFailure} {
		t.Run(string(outcome), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			sent := []byte{0, 255, 128, 1}
			response := []byte{254, 0, 129}
			stream := &applicationStreamFixture{
				reader: bytes.NewReader(response),
				eof:    make(chan struct{}),
				done:   make(chan endpointstream.Outcome, 1),
				result: outcome,
			}
			output := new(streamOutputFixture)
			err := exchangeStream(ctx, stream, io.NopCloser(bytes.NewReader(sent)), output)
			if (err == nil) != (outcome == endpointstream.CleanClose) {
				t.Fatalf("result = %v for %s", err, outcome)
			}
			if !bytes.Equal(stream.received.Bytes(), sent) || !bytes.Equal(output.Bytes(), response) {
				t.Fatalf("unexpected byte exchange: %x, %x", stream.received.Bytes(), output.Bytes())
			}
		})
	}
}

type applicationStreamFixture struct {
	reader   *bytes.Reader
	received bytes.Buffer
	eof      chan struct{}
	done     chan endpointstream.Outcome
	result   endpointstream.OutcomeClass
}

func (stream *applicationStreamFixture) Read(body []byte) (int, error) {
	<-stream.eof
	return stream.reader.Read(body)
}

func (stream *applicationStreamFixture) Write(body []byte) (int, error) {
	return stream.received.Write(body)
}

func (stream *applicationStreamFixture) CloseInput() error {
	close(stream.eof)
	stream.done <- endpointstream.Outcome{Class: stream.result}
	return nil
}

func (stream *applicationStreamFixture) Close() error { return nil }

func (stream *applicationStreamFixture) Done() <-chan endpointstream.Outcome { return stream.done }

type streamOutputFixture struct{ bytes.Buffer }

func (*streamOutputFixture) Close() error { return nil }

func TestApplicationStreamTerminalInterruptsIdleInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	input, writer := io.Pipe()
	defer writer.Close()
	response := []byte{255, 0, 128}
	stream := &applicationStreamFixture{
		reader: bytes.NewReader(response),
		eof:    make(chan struct{}),
		done:   make(chan endpointstream.Outcome, 1),
	}
	close(stream.eof)
	stream.done <- endpointstream.Outcome{Class: endpointstream.CleanClose}

	output := new(streamOutputFixture)
	if err := exchangeStream(ctx, stream, input, output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), response) {
		t.Fatalf("terminal discarded output: %x", output.Bytes())
	}
}

func TestApplicationStreamTerminalInterruptsBlockedWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	stream := &streamWithBlockedWrite{
		applicationStreamFixture: applicationStreamFixture{
			reader: bytes.NewReader([]byte{42}),
			eof:    make(chan struct{}),
			done:   make(chan endpointstream.Outcome, 1),
		},
		writing: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	go func() {
		select {
		case <-stream.writing:
			close(stream.eof)
			stream.done <- endpointstream.Outcome{Class: endpointstream.CleanClose}
		case <-ctx.Done():
			close(stream.eof)
		}
	}()
	output := new(streamOutputFixture)
	if err := exchangeStream(ctx, stream, io.NopCloser(bytes.NewReader([]byte{255})), output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), []byte{42}) {
		t.Fatalf("terminal discarded output: %x", output.Bytes())
	}
}

type streamWithBlockedWrite struct {
	applicationStreamFixture
	writing chan struct{}
	closed  chan struct{}
}

func (stream *streamWithBlockedWrite) Write([]byte) (int, error) {
	close(stream.writing)
	<-stream.closed
	return 0, net.ErrClosed
}

func (stream *streamWithBlockedWrite) Close() error {
	close(stream.closed)
	return nil
}
