package transport_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

func TestSharedPeerFailureCannotHideListenerOrCleanupFailure(t *testing.T) {
	peer := transport.MarkSharedPeerFailure(context.DeadlineExceeded)
	otherPeer := transport.MarkSharedPeerFailure(errors.New("another refused handshake"))
	for _, err := range []error{peer, fmt.Errorf("accept: %w", peer), errors.Join(peer, otherPeer)} {
		if !transport.IsClosedSharedPeerFailure(err) {
			t.Fatal("exact refused handshake category lost")
		}
	}
	fatal := errors.New("listener or physical cleanup failed")
	for _, err := range []error{errors.Join(peer, fatal), errors.Join(fatal, peer), fmt.Errorf("accept: %w", errors.Join(peer, fatal))} {
		if transport.IsClosedSharedPeerFailure(err) {
			t.Fatal("unrelated failure hidden behind a refused peer")
		}
		if !errors.Is(err, fatal) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("original aggregate causes discarded")
		}
	}
	if transport.IsClosedSharedPeerFailure(nil) || transport.IsClosedSharedPeerFailure(context.DeadlineExceeded) {
		t.Fatal("unmarked failure invented a peer-handshake phase")
	}
}
