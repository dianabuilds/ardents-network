//go:build linux || darwin

package node

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestKnownFirstCauseDoesNotWaitForPhysicalJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &forwarding{nodeLifetime: nodeLifetime{caller: ctx, ctx: ctx, cancel: cancel, retired: errors.New("original retired"), done: make(chan struct{}), failure: failureRecord{started: time.Now(), serving: true}}}
	handle := &Forwarding{original: original}
	cancel()
	handle.Stop()
	observed := make(chan *FailureDiagnostic, 1)
	go func() { observed <- handle.FirstFailure() }()
	select {
	case fact := <-observed:
		if fact == nil || fact.Kind != "logical" || fact.Cause != "canceled" || fact.Phase != "serving" {
			t.Error("already known caller cause unavailable", fact)
		}
		close(original.done)
	case <-time.After(50 * time.Millisecond):
		t.Error("known first cause waited for physical join")
		close(original.done)
		<-observed // Join this diagnostic reader even on the RED path.
	}
}
