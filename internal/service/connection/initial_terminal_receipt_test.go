//go:build linux

package connection

import (
	"errors"
	"net"
	"sync"
	"testing"
)

// The peer authenticates and exchanges Terminal, but never sends its receipt.
// A successful carrier write alone cannot complete the authenticated stream.
func TestAuthenticatedStreamRejectsUnconfirmedTerminal(t *testing.T) {
	left, right := net.Pipe()
	clientConfig, clientIdentity := initialAuthenticationFixture(t, left, true)
	publisherConfig, publisherIdentity := initialAuthenticationFixture(t, right, false)
	var workers sync.WaitGroup
	t.Cleanup(func() { _ = left.Close(); _ = right.Close(); workers.Wait() })
	ready := make(chan error, 1)
	peerDone := make(chan error, 1)
	workers.Go(func() {
		_, err := NewAuthenticatedStream(publisherConfig, publisherIdentity)
		ready <- err
		if err != nil {
			peerDone <- err
			return
		}
		if err := Write(right, Record{Terminal: &Terminal{AttachmentGeneration: 1, Offset: 0}}); err != nil {
			peerDone <- err
			return
		}
		for {
			record, err := ReadStream(right)
			if err != nil {
				peerDone <- err
				return
			}
			if record.Terminal != nil {
				peerDone <- nil
				return
			}
		}
	})
	client, err := NewAuthenticatedStream(clientConfig, clientIdentity)
	if peerErr := <-ready; err != nil || peerErr != nil {
		t.Fatal(errors.Join(err, peerErr))
	}
	_, runErr := client.RunBounded(0, 0)
	if peerErr := <-peerDone; peerErr != nil {
		t.Fatal(peerErr)
	}
	if runErr == nil {
		t.Fatal("authenticated stream completed without peer Terminal receipt")
	}
	client.mu.Lock()
	acknowledged := client.terminalAcknowledgedGeneration
	client.mu.Unlock()
	if acknowledged != 0 {
		t.Fatal("missing receipt was fabricated")
	}
}
