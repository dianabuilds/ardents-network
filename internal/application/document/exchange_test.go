package document_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

type responseStream struct {
	input               *bytes.Reader
	request             bytes.Buffer
	done                chan endpointstream.Outcome
	inputClosed, closed bool
	closeErr            error
	closeOnce           sync.Once
}

func (stream *responseStream) Read(body []byte) (int, error)  { return stream.input.Read(body) }
func (stream *responseStream) Write(body []byte) (int, error) { return stream.request.Write(body) }
func (stream *responseStream) CloseInput() error              { stream.inputClosed = true; return nil }
func (stream *responseStream) Close() error {
	stream.closeOnce.Do(func() { stream.closed = true })
	return stream.closeErr
}
func (stream *responseStream) Done() <-chan endpointstream.Outcome { return stream.done }

func TestReadRequiresExactResponseAndAuthenticatedCompletion(t *testing.T) {
	valid := []byte{0, 0xff, 0x80, 'x'}
	for _, test := range []struct {
		name     string
		response []byte
		outcome  string
		cleanup  error
		want     bool
	}{
		{"binary", valid, string(endpointstream.CleanClose), nil, true},
		{"empty", nil, string(endpointstream.CleanClose), nil, true},
		{"maximum", bytes.Repeat([]byte{0xff}, document.MaximumBytes), string(endpointstream.CleanClose), nil, true},
		{"oversize", bytes.Repeat([]byte{0xff}, document.MaximumBytes+1), string(endpointstream.CleanClose), nil, false},
		{"missing terminal", valid, "", nil, false},
		{"failed terminal", valid, "interrupted", nil, false},
		{"cleanup failure", valid, string(endpointstream.CleanClose), io.ErrClosedPipe, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := &responseStream{input: bytes.NewReader(test.response), done: make(chan endpointstream.Outcome, 1), closeErr: test.cleanup}
			if test.outcome != "" {
				stream.done <- endpointstream.Outcome{Class: endpointstream.OutcomeClass(test.outcome)}
			}
			close(stream.done)
			body, err := document.Read(context.Background(), stream)
			if (err == nil) != test.want || (!test.want && body != nil) {
				t.Fatalf("read = %q, %v", body, err)
			}
			if !stream.closed || !stream.inputClosed || stream.request.Len() != 0 {
				t.Fatal("one-request/cleanup contract was lost")
			}
			if test.want && string(body) != string(test.response) {
				t.Fatal("returned different content")
			}
		})
	}
}

func TestCancelledReadCreatesNoRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := &responseStream{input: bytes.NewReader(nil), done: make(chan endpointstream.Outcome)}
	_, err := document.Read(ctx, stream)
	if !errors.Is(err, context.Canceled) || stream.request.Len() != 0 || !stream.closed {
		t.Fatalf("cancelled read = %v", err)
	}
}
