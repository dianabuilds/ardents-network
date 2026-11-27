package document_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/document"
)

type documentOutput struct {
	bytes.Buffer
	closed bool
}

func (o *documentOutput) Close() error { o.closed = true; return nil }

type observedInput struct {
	io.ReadCloser
	once    sync.Once
	entered chan struct{}
}

func (input *observedInput) Read(p []byte) (int, error) {
	input.once.Do(func() { close(input.entered) })
	return input.ReadCloser.Read(p)
}

func TestDocumentReadCancelsAndJoinsPendingHumanInput(t *testing.T) {
	reader, pending := io.Pipe()
	defer pending.Close()
	defer reader.Close()
	input := &observedInput{ReadCloser: reader, entered: make(chan struct{})}
	output := &documentOutput{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- document.ReadLocal(ctx, filepath.Join(t.TempDir(), "absent.sock"), input, output) }()
	select {
	case <-input.entered:
	case err := <-done:
		t.Fatalf("read returned before admission: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("input read did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || output.Len() != 0 || !output.closed {
			t.Fatal("cancelled input emitted content or retained its handles")
		}
		writeDone := make(chan error, 1)
		go func() {
			_, err := pending.Write([]byte("fixture"))
			writeDone <- err
		}()
		select {
		case err := <-writeDone:
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatal("original input remained open", err)
			}
		case <-time.After(3 * time.Second):
			_ = reader.Close()
			<-writeDone
			t.Fatal("original input remained open")
		}
	case <-time.After(3 * time.Second):
		_ = reader.Close()
		<-done
		t.Fatal("input cancellation did not join")
	}
}
