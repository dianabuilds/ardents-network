//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
)

type textLinkPeer struct{ publicationPeer }

func (*textLinkPeer) PublishedLink(context.Context) (string, error) {
	return "ardents://test-public-link", nil
}

type signaledLinkOutput struct {
	*io.PipeWriter
	entered chan struct{}
}

func (output *signaledLinkOutput) Write(value []byte) (int, error) {
	close(output.entered)
	return output.PipeWriter.Write(value)
}

func TestPublishedLinkPresentationAndCancellation(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "admin.sock")
	peer := &textLinkPeer{}
	server, err := administration.Listen(socket, peer)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, blocked := range []bool{false, true} {
		input, output := io.Pipe()
		entered := make(chan struct{})
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		var helpers sync.WaitGroup
		helpers.Go(func() { done <- showPublishedLink(ctx, socket, &signaledLinkOutput{output, entered}) })
		t.Cleanup(func() {
			cancel()
			if err := errors.Join(input.Close(), output.Close()); err != nil {
				t.Error(err)
			}
			helpers.Wait()
		})
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			cancel()
			input.Close()
			<-done
			t.Fatal("Link did not reach presentation")
		}
		if blocked {
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled output: %v", err)
			}
		} else {
			raw, err := io.ReadAll(input)
			if err != nil || string(raw) != "ardents://test-public-link\n" {
				t.Fatalf("Link presentation: %q %v", raw, err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		}
		cancel()
		if err := input.Close(); err != nil {
			t.Error(err)
		}
	}
	if peer.calls.Load() != 0 {
		t.Fatal("Link presentation published a snapshot")
	}
}
