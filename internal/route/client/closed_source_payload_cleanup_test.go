//go:build linux

package client

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// Retirement must join a frame whose header has already reached the peer.
// A local cancellation is not permission to cut that frame in half when it can
// finish within its original authority and the existing finite cleanup bound.
func TestClosedSourceCloseJoinsEmittedPayloadWithinCleanupBound(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		name := "peer-drains"
		if stalled {
			name = "peer-stalls"
		}
		t.Run(name, func(t *testing.T) {
			owner, peer, end := sourceChannelsFixture(t)
			opened := make(chan error, 1)
			go func() { _, err := ardp.ReadFrame(peer); opened <- err }()
			lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
			if err != nil {
				t.Fatal(err)
			}
			if err := <-opened; err != nil {
				t.Fatal(err)
			}
			body := []byte("already admitted payload")
			written := make(chan error, 1)
			go func() { _, err := lane.Write(body); written <- err }()
			var first [1]byte
			if _, err := io.ReadFull(peer, first[:]); err != nil {
				t.Fatal(err)
			}
			closed := make(chan error, 1)
			go func() { closed <- lane.Close() }()
			waitSourceChannelState(t, owner, func() bool { return lane.closed })
			if stalled {
				select {
				case closeErr := <-closed:
					writeErr := <-written
					if writeErr == nil || !errors.Is(closeErr, writeErr) || !errors.Is(closeErr, ErrClosedSourceCleanup) {
						t.Fatalf("lost failed active payload: write=%v cleanup=%v", writeErr, closeErr)
					}
				case <-time.After(1500 * time.Millisecond):
					t.Fatal("stalled payload exceeded cleanup bound")
				}
				select {
				case <-owner.done:
				case <-time.After(time.Second):
					t.Fatal("failed payload did not join its owner")
				}
				return
			}
			select {
			case err := <-written:
				t.Fatalf("retirement cut an emitted payload before cleanup bound: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			rest := make([]byte, ardp.HeaderSize+len(body)-1)
			if _, err := io.ReadFull(peer, rest); err != nil {
				t.Fatal(err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
			terminal, err := ardp.ReadFrame(peer)
			if err != nil || terminal.Kind != ardp.KindClose || terminal.Lane != lane.id || len(terminal.Body) != 1 || terminal.Body[0] != lane.closeStatus {
				t.Fatalf("payload retirement did not preserve terminal frame: frame=%+v err=%v", terminal, err)
			}
			if err := <-closed; err != nil {
				t.Fatal(err)
			}
		})
	}
}
