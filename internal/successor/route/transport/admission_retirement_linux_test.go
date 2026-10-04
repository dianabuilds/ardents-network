//go:build linux

package transport

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdmissionRollbackRetainsCapacityUntilPhysicalJoin(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.readGate = make(chan struct{})
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	lifetime := &admissionRetirement{}
	releaseFailure := errors.New("retained capacity release failed")
	var releases atomic.Int32
	rollback := (Channel{capacity: lifetime}).HoldReservation(func() error {
		select {
		case <-s.readerDone:
		default:
			t.Error("capacity released before physical reader joined")
		}
		releases.Add(1)
		return releaseFailure
	})
	// Receiving Admission has refused after acquiring capacity; no successful
	// Grant is invented by this lifecycle oracle.
	if err := rollback(); err != nil {
		t.Fatal(err)
	}
	if releases.Load() != 0 {
		t.Fatal("rollback bypassed physical retirement")
	}
	result := make(chan error, 1)
	go func() { result <- errors.Join(s.Close(), lifetime.finish()) }()
	<-physical.closed
	if releases.Load() != 0 {
		t.Fatal("socket Close substituted for reader join")
	}
	close(physical.readGate)
	if err := lifecycleResult(t, result); !errors.Is(err, releaseFailure) {
		t.Fatal("joined release failure lost", err)
	}
	if err := rollback(); !errors.Is(err, releaseFailure) || releases.Load() != 1 {
		t.Fatal("repeated release changed result or retried mutation", err)
	}
}
