package forwarding

import (
	"context"
	"testing"
	"time"
)

func TestForwardingTransitionCoordinatorRefusesForeignActionsBeforeInheritedFDs(t *testing.T) {
	t.Setenv("ARDENTS_FORWARDING_TRANSITION_CHILD", "coordinator-v1")
	t.Setenv("ARDENTS_FORWARDING_TRANSITION_FD", "4")
	t.Setenv("ARDENTS_FORWARDING_TRANSITION_NONCE", digest([]byte("finite nonce fixture")))
	for _, action := range []string{"", "start-installed", "inspect-inputs", "inspect-start-history", "source", "node", "recover", "supervise-transition", "/bin/sh"} {
		coordinator, selected, err := OpenTransitionCoordinator(action)
		if !selected || coordinator != nil || err != ErrInput {
			t.Fatal("foreign action reached inherited descriptors", action, selected, err)
		}
	}
	t.Setenv("ARDENTS_FORWARDING_TRANSITION_CHILD", "foreign")
	for _, action := range []string{"install-initial-stopped", "replace", "start-initial", "restart", "start-selected", "stop"} {
		coordinator, selected, err := OpenTransitionCoordinator(action)
		if !selected || coordinator != nil || err != ErrInput {
			t.Fatal("malformed marker selected ordinary admission", action, selected, err)
		}
	}
}

func TestForwardingTransitionContextKeepsOriginalControllerDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	base := transitionDeadline{Context: context.Background(), deadline: deadline}
	coordinator := &TransitionCoordinator{}
	original := context.WithValue(base, transitionContextKey{}, coordinator)
	child, cancel := TransitionContext(original, time.Nanosecond)
	cancel()
	actual, bounded := child.Deadline()
	if child != original || !bounded || !actual.Equal(deadline) || child.Err() != nil {
		t.Fatal("child introduced a competing cleanup timer")
	}
	ordinary, finish := TransitionContext(context.Background(), time.Minute)
	finish()
	if ordinary.Err() != context.Canceled {
		t.Fatal("ordinary command lost owned cancellation")
	}
	// This detached timing check grants no original peer, Release or cut proof.
}
