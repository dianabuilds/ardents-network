package document_test

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func TestReaderWorkerUsesOneAdmittedServiceStreamAndJoinsItsResult(t *testing.T) {
	for _, size := range []int{0, 64 << 10, 160 << 10, document.MaximumBytes} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), size)
			response := body
			stream := &responseStream{input: bytes.NewReader(response), done: make(chan connection.Outcome, 1)}
			stream.done <- connection.Outcome{Class: connection.CleanClose}
			close(stream.done)
			worker, _ := startDocumentWorker(t, document.ReaderWorker, nil)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			actual, err := readTransferredReader(ctx, worker, stream)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, body) || !stream.closed || !stream.inputClosed || stream.request.Len() != 0 {
				t.Fatal("worker result or one-request joined lifecycle was lost")
			}

		})
	}
}

func TestReaderWorkerNeverReturnsContentOnInterruptedServiceOrCleanup(t *testing.T) {
	for _, test := range []struct {
		name     string
		terminal connection.OutcomeClass
		cleanup  error
	}{
		{"missing terminal", "", nil}, {"non-clean terminal", connection.LocalFailure, nil}, {"cleanup failure", connection.CleanClose, io.ErrClosedPipe},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := &responseStream{input: bytes.NewReader([]byte("ok")), done: make(chan connection.Outcome, 1), closeErr: test.cleanup}
			if test.terminal != "" {
				stream.done <- connection.Outcome{Class: test.terminal}
			}
			close(stream.done)
			worker, _ := startDocumentWorker(t, document.ReaderWorker, nil)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			body, err := readTransferredReader(ctx, worker, stream)
			if err == nil || body != nil || !stream.closed {
				t.Fatal("failed Connection or cleanup returned content")
			}
		})
	}
}
