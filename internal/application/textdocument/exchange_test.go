package textdocument_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"io"
	"sync"
	"testing"
)

type responseStream struct {
	input               *bytes.Reader
	request             bytes.Buffer
	done                chan connection.Outcome
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
func (stream *responseStream) Done() <-chan connection.Outcome { return stream.done }

func TestReadRequiresExactResponseAndAuthenticatedCompletion(t *testing.T) {
	valid := []byte("ARDTXT01\x00\x00\x00\x00\x02ok")
	oversize := bytes.Clone(valid)
	binary.BigEndian.PutUint32(oversize[9:], textdocument.MaximumBytes+1)
	for _, test := range []struct {
		name     string
		response []byte
		outcome  string
		cleanup  error
		want     bool
	}{
		{"success", valid, string(connection.CleanClose), nil, true},
		{"empty", []byte("ARDTXT01\x00\x00\x00\x00\x00"), string(connection.CleanClose), nil, true},
		{"missing terminal", valid, "", nil, false},
		{"failed terminal", valid, "interrupted", nil, false},
		{"truncated", valid[:14], string(connection.CleanClose), nil, false},
		{"trailing", append(bytes.Clone(valid), 'x'), string(connection.CleanClose), nil, false},
		{"unavailable", []byte("ARDTXT01\x01\x00\x00\x00\x00"), string(connection.CleanClose), nil, false},
		{"unknown status", []byte("ARDTXT01\x02\x00\x00\x00\x00"), string(connection.CleanClose), nil, false},
		{"oversize", oversize, string(connection.CleanClose), nil, false},
		{"invalid UTF-8", []byte("ARDTXT01\x00\x00\x00\x00\x01\xff"), string(connection.CleanClose), nil, false},
		{"cleanup failure", valid, string(connection.CleanClose), io.ErrClosedPipe, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := &responseStream{input: bytes.NewReader(test.response), done: make(chan connection.Outcome, 1), closeErr: test.cleanup}
			if test.outcome != "" {
				stream.done <- connection.Outcome{Class: connection.OutcomeClass(test.outcome)}
			}
			close(stream.done)
			body, err := textdocument.Read(context.Background(), stream)
			if (err == nil) != test.want || (!test.want && body != nil) {
				t.Fatalf("read = %q, %v", body, err)
			}
			if !stream.closed || !stream.inputClosed || stream.request.Len() != 512 {
				t.Fatal("one-request/cleanup contract was lost")
			}
			if test.want && string(body) != string(test.response[13:]) {
				t.Fatal("returned different content")
			}
		})
	}
}

func TestCancelledReadCreatesNoRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := &responseStream{input: bytes.NewReader(nil), done: make(chan connection.Outcome)}
	_, err := textdocument.Read(ctx, stream)
	if !errors.Is(err, context.Canceled) || stream.request.Len() != 0 || !stream.closed {
		t.Fatalf("cancelled read = %v", err)
	}
}
