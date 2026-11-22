//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func documentReadSocket(t *testing.T, peer connection.Interface) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "document-read-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(dir); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(dir, "local.sock")
	server, err := connection.Listen(socket, peer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return socket
}

func TestDocumentReadUsesOneAAI3ConnectionAndSafePresentation(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("safe\n\x1b[31m\u202etext\t\u0085"), bytes.Repeat([]byte("x"), document.MaximumBytes)} {
		peer := &documentReadPeer{response: documentResponse(body), class: connection.CleanClose}
		socket := documentReadSocket(t, peer)
		var want bytes.Buffer
		if err := document.WritePlainText(&want, body); err != nil {
			t.Fatal(err)
		}
		output := &documentOutput{}
		err := readDocument(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output)
		if err != nil || !bytes.Equal(output.Bytes(), want.Bytes()) || !output.closed ||
			peer.opens.Load() != 1 || peer.requestBytes.Load() != 512 {
			t.Fatalf("one complete UI read failed: %v, opens=%d, request=%d", err, peer.opens.Load(), peer.requestBytes.Load())
		}
	}
}

func TestDocumentReadNeverPresentsPartialOrFailedConnection(t *testing.T) {
	for _, test := range []struct {
		response []byte
		class    connection.OutcomeClass
	}{
		{documentResponse([]byte("secret")), connection.LocalFailure},
		{append(documentResponse([]byte("secret")), 0), connection.CleanClose},
		{documentResponse([]byte("secret"))[:15], connection.CleanClose},
		{documentResponse([]byte{0xff}), connection.CleanClose},
	} {
		peer := &documentReadPeer{response: test.response, class: test.class}
		socket := documentReadSocket(t, peer)
		output := &documentOutput{}
		if err := readDocument(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); err == nil ||
			output.Len() != 0 || peer.opens.Load() != 1 || !output.closed {
			t.Fatal("failed response produced output or retried")
		}
	}
}

func TestDocumentReadPreservesRefusedSetupClassWithoutPresentingItsReason(t *testing.T) {
	for _, test := range []struct {
		class connection.OutcomeClass
		want  string
	}{
		{connection.ServiceUnavailable, "document operation unavailable"},
		{connection.LocalFailure, "local connection failed"},
		{connection.IndeterminateFailure, "connection outcome unknown"},
		{connection.CapacityUnavailable, "document capacity unavailable"},
		{connection.LocalCancellation, "read cancelled"},
		{connection.LocalTimeout, "read timed out"},
		{connection.CleanClose, "document operation unavailable"},
		{"fixture private class", "document operation unavailable"},
	} {
		t.Run(string(test.class), func(t *testing.T) {
			peer := &refusingDocumentReadPeer{outcome: connection.Outcome{Class: test.class, Reason: "private fixture detail"}}
			socket := documentReadSocket(t, peer)
			output := &documentOutput{}
			err := readDocument(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output)
			var refusal connection.SetupRefusalError
			if !errors.As(err, &refusal) || refusal.Outcome().Class != test.class ||
				output.Len() != 0 || !output.closed || peer.opens.Load() != 1 || documentFailure(err) != test.want {
				t.Fatalf("refused setup = %v, output=%d, closed=%t, opens=%d", err, output.Len(), output.closed, peer.opens.Load())
			}
		})
	}
}

func TestDocumentReadLocalContextDiagnosticPrecedesDeliveredRefusal(t *testing.T) {
	refusal := connection.Refuse(connection.Outcome{Class: connection.CapacityUnavailable, Reason: "private fixture detail"})
	for _, test := range []struct {
		contextErr error
		want       string
	}{
		{context.Canceled, "read cancelled"},
		{context.DeadlineExceeded, "read timed out"},
	} {
		if got := documentFailure(errors.Join(refusal, test.contextErr)); got != test.want {
			t.Fatalf("local context diagnostic = %q, want %q", got, test.want)
		}
	}
}

func TestDocumentReadCancellationDuringSetupJoinsWithoutPresentation(t *testing.T) {
	peer := &setupBlockingDocumentReadPeer{entered: make(chan struct{})}
	socket := documentReadSocket(t, peer)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &documentOutput{}
	done := make(chan error, 1)
	go func() { done <- readDocument(ctx, socket, io.NopCloser(strings.NewReader("fixture-link\n")), output) }()
	select {
	case <-peer.entered:
	case <-time.After(time.Second):
		t.Fatal("reader did not reach setup")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || output.Len() != 0 || !output.closed {
			t.Fatalf("cancelled setup = %v, output=%d, closed=%t", err, output.Len(), output.closed)
		}
	case <-time.After(time.Second):
		t.Fatal("reader setup cancellation did not join")
	}
}

func TestDocumentReadWaitsForTerminalAndJoinsCancellation(t *testing.T) {
	peer := &documentReadPeer{response: documentResponse([]byte("secret")), class: connection.CleanClose,
		terminal: make(chan struct{}), responseSent: make(chan struct{})}
	socket := documentReadSocket(t, peer)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &documentOutput{}
	done := make(chan error, 1)
	go func() { done <- readDocument(ctx, socket, io.NopCloser(strings.NewReader("fixture-link\n")), output) }()
	select {
	case <-peer.responseSent:
	case <-time.After(3 * time.Second):
		t.Fatal("fixture response was not consumed")
	}
	select {
	case <-done:
		t.Fatal("UI completed before terminal outcome")
	default:
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || output.Len() != 0 || !output.closed || peer.opens.Load() != 1 {
			t.Fatal("cancellation emitted content or retained UI handles")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("read cancellation did not join")
	}
}

func TestDocumentReadPreservesPresentationCleanupFailure(t *testing.T) {
	peer := &documentReadPeer{response: documentResponse([]byte("ok")), class: connection.CleanClose}
	socket := documentReadSocket(t, peer)
	output := &documentOutput{err: io.ErrClosedPipe}
	if err := readDocument(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); !errors.Is(err, io.ErrClosedPipe) || !output.closed {
		t.Fatal("UI reported success after presentation close failed")
	}
}
