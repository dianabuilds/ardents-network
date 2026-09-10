package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTextDestinationIsBoundedAndNeverRewritten(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"fixture-link\n", "fixture-link"}, {"fixture-link\r\n", "fixture-link"},
		{"fixture-link", "fixture-link"}, {strings.Repeat("x", 512) + "\r\n", strings.Repeat("x", 512)},
		{" first link \nsecond\n", " first link "},
	} {
		got, err := readTextDestination(strings.NewReader(test.input))
		if err != nil || got != test.want {
			t.Fatalf("destination = %q, %v", got, err)
		}
	}
	for _, input := range []string{"", "\n", "\r\n", "\xff\n", strings.Repeat("x", 513), strings.Repeat("x", 100000)} {
		if _, err := readTextDestination(strings.NewReader(input)); !errors.Is(err, errTextInput) {
			t.Fatal("unbounded or invalid destination accepted")
		}
	}
}

func TestTextReadCancelsAndJoinsPendingHumanInput(t *testing.T) {
	input, pending := io.Pipe()
	defer pending.Close()
	output := &textOutput{}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- readText(ctx, filepath.Join(t.TempDir(), "absent.sock"), input, output) }()
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

func TestTextReadInvalidInputProducesNoOutput(t *testing.T) {
	output := &textOutput{}
	err := readText(t.Context(), filepath.Join(t.TempDir(), "absent.sock"), io.NopCloser(strings.NewReader("\n")), output)
	if !errors.Is(err, errTextInput) || output.Len() != 0 || !output.closed {
		t.Fatal("invalid input was not refused before attachment")
	}
	for _, input := range [][]string{{"read"}, {"read", "relative.sock"}, {"read", "/socket", "/output-file"}} {
		if err := run(input); err == nil {
			t.Fatal("unsupported read invocation accepted")
		}
	}
}

func TestTextDiagnosticsContainOnlyLocalClass(t *testing.T) {
	secret := errors.New("fixture destination or private response")
	for _, test := range []struct {
		err  error
		want string
	}{
		{secret, "text operation unavailable"},
		{errors.Join(secret, context.Canceled), "text read cancelled"},
		{errors.Join(secret, context.DeadlineExceeded), "text read timed out"},
		{errors.Join(secret, errTextInput), "text destination or local input is invalid"},
	} {
		if got := textFailure(test.err); got != test.want {
			t.Fatal("diagnostic leaked remote or local input")
		}
	}
}

type textOutput struct {
	bytes.Buffer
	closed bool
	err    error
}

func (output *textOutput) Close() error { output.closed = true; return output.err }
