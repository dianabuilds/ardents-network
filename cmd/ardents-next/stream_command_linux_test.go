//go:build linux

package main

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

func TestApplicationStreamCopiesBytesAndRequiresAuthenticatedCompletion(t *testing.T) {
	for _, outcome := range []connection.OutcomeClass{connection.CleanClose, connection.IndeterminateFailure} {
		t.Run(string(outcome), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			sent := []byte{0, 255, 128, 1}
			response := []byte{254, 0, 129}
			stream := &applicationStreamFixture{
				reader: bytes.NewReader(response),
				eof:    make(chan struct{}),
				done:   make(chan connection.Outcome, 1),
				result: outcome,
			}
			output := new(streamOutputFixture)
			err := exchangeApplicationStream(ctx, stream, io.NopCloser(bytes.NewReader(sent)), output)
			if (err == nil) != (outcome == connection.CleanClose) {
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
	done     chan connection.Outcome
	result   connection.OutcomeClass
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
	stream.done <- connection.Outcome{Class: stream.result}
	return nil
}

func (stream *applicationStreamFixture) Close() error { return nil }

func (stream *applicationStreamFixture) Done() <-chan connection.Outcome { return stream.done }

type streamOutputFixture struct{ bytes.Buffer }

func (*streamOutputFixture) Close() error { return nil }
