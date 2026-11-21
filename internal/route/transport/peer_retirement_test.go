package transport

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestPeerRetirementMarkerCannotHideAggregateFailure(t *testing.T) {
	peer := MarkPeerRetirement(errors.New("exact adapter observation"))
	foreign := errors.New("independent physical failure")
	for _, err := range []error{peer, fmt.Errorf("physical: %w", peer), errors.Join(peer, syscall.EPIPE)} {
		if !IsPeerRetirementCause(err) {
			t.Fatal("peer observation lost", err)
		}
	}
	for _, err := range []error{errors.Join(peer, foreign), fmt.Errorf("physical: %w", errors.Join(foreign, peer))} {
		marked := MarkPeerRetirement(err)
		if IsPeerRetirementCause(marked) || !errors.Is(marked, foreign) {
			t.Fatal("marker hid independent aggregate cause", marked)
		}
	}
	if MarkPeerRetirement(nil) != nil {
		t.Fatal("nil invented a failure")
	}
}
