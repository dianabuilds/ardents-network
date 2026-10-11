package runtime

import (
	"errors"
	"testing"
)

// This tests only the retained read observation; it supplies no qualified
// launch, Job, operation, worker join or completed-current authority.
func TestInvocationCleanupResultWaitsForOriginalCompletion(t *testing.T) {
	var absent *Invocation
	if err, completed := absent.CleanupResult(); err != nil || completed {
		t.Fatal("absent invocation supplied completed cleanup", err)
	}
	physical := errors.New("original physical cleanup failure")
	use := errors.New("separate original worker use failure")
	invocation := &Invocation{done: make(chan struct{}), cleanupResult: physical, result: errors.Join(physical, use), observationError: use}
	if err, completed := invocation.CleanupResult(); err != nil || completed {
		t.Fatal("unjoined invocation exposed a terminal cleanup result", err)
	}
	close(invocation.done)
	for range 2 {
		if err, completed := invocation.CleanupResult(); err != physical || !completed {
			t.Fatal("original physical failure was lost or replaced", err)
		}
		if err := invocation.Close(); !errors.Is(err, physical) || !errors.Is(err, use) {
			t.Fatal("complete retirement lost cleanup or worker failure", err)
		}
	}
}
