package document_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/document"
)

type documentOutput struct {
	bytes.Buffer
	closed bool
}

func (o *documentOutput) Close() error { o.closed = true; return nil }

func TestDocumentReadCancelsAndJoinsPendingHumanInput(t *testing.T) {
	input, pending := io.Pipe()
	defer pending.Close()
	output := &documentOutput{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- document.ReadLocal(ctx, filepath.Join(t.TempDir(), "absent.sock"), input, output) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || output.Len() != 0 || !output.closed {
			t.Fatal("cancelled input emitted content or retained its handles")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("input cancellation did not join")
	}
}
