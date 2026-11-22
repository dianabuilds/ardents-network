package join

import (
	"context"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	"net"
	"testing"
	"time"
)

// This fixture starts only real physical framing; it supplies no authority,
// admission or successful receiving result.
func newJoinedSession(ctx context.Context, conn net.Conn, end time.Time, remaining uint64, check func() error, queues *framing.Budget) (*framing.Session, *framing.Lane, error) {
	s, l, err := framing.PrepareJoined(ctx, conn, end, remaining, check, queues)
	if err != nil {
		return nil, nil, err
	}
	s.Start()
	return s, l, nil
}

// Filling the expected remainder detects retained memory in either direction;
// zero-memory claims independently detect an early or leaked child return.
func assertRetainedBudget(t *testing.T, budget *framing.Budget, maximum, used uint64, children int) {
	t.Helper()
	remainder, err := budget.HoldChild(maximum - used)
	if err != nil {
		t.Fatal("expected remaining principal memory unavailable", err)
	}
	defer remainder()
	if extra, err := budget.HoldChild(1); err == nil {
		extra()
		t.Fatal("principal memory returned before its owner joined")
	}
	var claims []func()
	defer func() {
		for _, release := range claims {
			release()
		}
	}()
	for range 1023 - children {
		release, err := budget.HoldChild(0)
		if err != nil {
			t.Fatal("expected remaining child capacity unavailable", err)
		}
		claims = append(claims, release)
	}
	if extra, err := budget.HoldChild(0); err == nil {
		extra()
		t.Fatal("child returned before its owner joined")
	}
}
