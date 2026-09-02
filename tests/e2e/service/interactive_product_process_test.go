package service_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

type interactiveProductInput struct {
	prompt string
	value  string
	secret bool
}

type interactiveProductCapture struct {
	mu     sync.Mutex
	output bytes.Buffer
}

func (capture *interactiveProductCapture) Write(value []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.output.Write(value)
}

func (capture *interactiveProductCapture) bytes() []byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]byte(nil), capture.output.Bytes()...)
}

func runInteractiveProductCommand(t *testing.T, directory, binary string, inputs []interactiveProductInput, arguments ...string) []byte {
	t.Helper()
	output, err := runInteractiveProductCommandResult(t, directory, binary, inputs, arguments...)
	if err != nil {
		t.Fatalf("interactive product command failed: %v\n%s", err, output)
	}
	return output
}

func runInteractiveProductCommandResult(t *testing.T, directory, binary string, inputs []interactiveProductInput, arguments ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	terminal, err := pty.New()
	if err != nil {
		t.Fatalf("open product pseudo-terminal: %v", err)
	}
	command := terminal.CommandContext(ctx, binary, arguments...)
	command.Dir = directory
	if err := command.Start(); err != nil {
		_ = terminal.Close()
		t.Fatalf("start interactive product command: %v", err)
	}
	capture := new(interactiveProductCapture)
	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(capture, terminal)
		copyDone <- copyErr
	}()
	for _, input := range inputs {
		if err := waitForInteractivePrompt(ctx, capture, input.prompt); err != nil {
			_ = terminal.Close()
			_ = command.Wait()
			t.Fatalf("interactive product prompt: %v\n%s", err, capture.bytes())
		}
		if input.secret {
			if err := waitForInteractiveNoEcho(ctx, terminal); err != nil {
				_ = terminal.Close()
				_ = command.Wait()
				t.Fatalf("interactive product password terminal: %v\n%s", err, capture.bytes())
			}
		}
		if _, err := io.WriteString(terminal, input.value+interactiveProductLineEnding(input.secret)); err != nil {
			_ = terminal.Close()
			_ = command.Wait()
			t.Fatalf("write interactive product input: %v", err)
		}
	}
	waitErr := command.Wait()
	closeErr := terminal.Close()
	select {
	case copyErr := <-copyDone:
		if copyErr != nil && !errors.Is(copyErr, io.EOF) && !errors.Is(copyErr, os.ErrClosed) &&
			!errors.Is(copyErr, errors.ErrUnsupported) && !isExpectedInteractiveProductReadError(copyErr) {
			t.Fatalf("read interactive product output: %v", copyErr)
		}
	case <-ctx.Done():
		t.Fatalf("interactive product output did not close: %v", ctx.Err())
	}
	output := capture.bytes()
	return output, errors.Join(waitErr, closeErr)
}

func waitForInteractivePrompt(ctx context.Context, capture *interactiveProductCapture, prompt string) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		if bytes.Contains(capture.bytes(), []byte(prompt)) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func interactiveProductJSON(t *testing.T, terminal []byte) []byte {
	t.Helper()
	start, end := bytes.IndexByte(terminal, '{'), bytes.LastIndexByte(terminal, '}')
	if start < 0 || end < start {
		t.Fatalf("interactive product output lacks one JSON receipt: %q", terminal)
	}
	return append([]byte(nil), terminal[start:end+1]...)
}
