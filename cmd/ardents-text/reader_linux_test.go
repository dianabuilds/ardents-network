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

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func textReadSocket(t *testing.T, peer *textReadPeer) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "text-read-")
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

func TestTextReadUsesOneAAI3ConnectionAndSafePresentation(t *testing.T) {
	for _, body := range [][]byte{nil, []byte("safe\n\x1b[31m\u202etext\t\u0085"), bytes.Repeat([]byte("x"), textdocument.MaximumBytes)} {
		peer := &textReadPeer{response: textResponse(body), class: connection.CleanClose}
		socket := textReadSocket(t, peer)
		var want bytes.Buffer
		if err := textdocument.WritePlainText(&want, body); err != nil {
			t.Fatal(err)
		}
		output := &textOutput{}
		err := readText(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output)
		if err != nil || !bytes.Equal(output.Bytes(), want.Bytes()) || !output.closed ||
			peer.opens.Load() != 1 || peer.requestBytes.Load() != 512 {
			t.Fatalf("one complete UI read failed: %v, opens=%d, request=%d", err, peer.opens.Load(), peer.requestBytes.Load())
		}
	}
}

func TestTextReadNeverPresentsPartialOrFailedConnection(t *testing.T) {
	for _, test := range []struct {
		response []byte
		class    connection.OutcomeClass
	}{
		{textResponse([]byte("secret")), connection.LocalFailure},
		{append(textResponse([]byte("secret")), 0), connection.CleanClose},
		{textResponse([]byte("secret"))[:15], connection.CleanClose},
		{textResponse([]byte{0xff}), connection.CleanClose},
	} {
		peer := &textReadPeer{response: test.response, class: test.class}
		socket := textReadSocket(t, peer)
		output := &textOutput{}
		if err := readText(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); err == nil ||
			output.Len() != 0 || peer.opens.Load() != 1 || !output.closed {
			t.Fatal("failed response produced output or retried")
		}
	}
}

func TestTextReadWaitsForTerminalAndJoinsCancellation(t *testing.T) {
	peer := &textReadPeer{response: textResponse([]byte("secret")), class: connection.CleanClose,
		terminal: make(chan struct{}), responseSent: make(chan struct{})}
	socket := textReadSocket(t, peer)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &textOutput{}
	done := make(chan error, 1)
	go func() { done <- readText(ctx, socket, io.NopCloser(strings.NewReader("fixture-link\n")), output) }()
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

func TestTextReadPreservesPresentationCleanupFailure(t *testing.T) {
	peer := &textReadPeer{response: textResponse([]byte("ok")), class: connection.CleanClose}
	socket := textReadSocket(t, peer)
	output := &textOutput{err: io.ErrClosedPipe}
	if err := readText(t.Context(), socket, io.NopCloser(strings.NewReader("fixture-link\n")), output); !errors.Is(err, io.ErrClosedPipe) || !output.closed {
		t.Fatal("UI reported success after presentation close failed")
	}
}
