//go:build linux

package hosting

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMeasurementCannotTransferExpiredReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	deadline := now.Add(time.Second)
	owner.measure = func([]string) (hostingReading, error) {
		*now = deadline
		return *reading, nil
	}
	held, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 10}, Termination: Traffic{Rx: 10}, WorkUntil: deadline, HoldUntil: deadline})
	if err == nil || held != nil {
		t.Fatal("measurement delay transferred expired reservation")
	}
	view, err := owner.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatalf("late admission changed budget: %+v / %v", view, err)
	}
}

func TestPostCommitExpiryOnlyReleasesOwnReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	other, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 20}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	deadline := now.Add(time.Second)
	// The durable state transition, not clock-call counts, advances the clock.
	owner.now = func() time.Time {
		state, err := readCommittedHostingState(owner.root)
		if err != nil {
			t.Fatal(err)
		}
		if state.Reserved == 50 {
			*now = deadline
		}
		return *now
	}
	held, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 10}, Termination: Traffic{Rx: 10}, WorkUntil: deadline, HoldUntil: deadline.Add(time.Second)})
	if err == nil || held != nil {
		t.Fatal("late commit transferred work")
	}
	view, err := owner.Observe(t.Context())
	if err != nil || view.ReservedBytes != 30 {
		t.Fatal("late cleanup changed another reserve", view, err)
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionCoverageDoesNotRenewWork(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	held, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 10}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Second), HoldUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(2 * time.Second)
	view, err := owner.Observe(t.Context())
	if err != nil || view.ReservedBytes != 20 {
		t.Fatal("expiry released unfinished work", view, err)
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 10}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: *now}); err == nil {
		t.Fatal("cleanup ends before work")
	}
}

func TestPostCommitCancellationDoesNotTransferReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner.now = func() time.Time {
		state, err := readCommittedHostingState(owner.root)
		if err != nil {
			t.Fatal(err)
		}
		if state.Reserved == 20 {
			cancel()
		}
		return *now
	}
	held, err := owner.Reserve(ctx, ReservationRequest{Work: Traffic{Tx: 10}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
	if held != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled commit transferred reservation", held, err)
	}
	view, err := owner.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatal("canceled unpublished reservation retained", view, err)
	}
}

func TestObservationRollbackDuringMeasurementRefusesCapacity(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	owner.measure = func([]string) (hostingReading, error) { *now = now.Add(-time.Second); return *reading, nil }
	view, err := owner.Observe(t.Context())
	if err == nil || !view.Protect || !view.Drain {
		t.Fatal("rollback returned usable observation", view, err)
	}
}

func TestCopiedBudgetSharesClosedLifecycle(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	independent := openHostingFixture(t, root, reading, now)
	copied := *owner
	if err := copied.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Observe(t.Context()); err == nil {
		t.Fatal("copy did not close shared owner")
	}
	if _, err := independent.Observe(t.Context()); err != nil {
		t.Fatal("copy closed independent owner", err)
	}
}

func TestIndependentOwnersCannotOversubscribe(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owners := make([]*Budget, 12)
	for i := range owners {
		owners[i] = openHostingFixture(t, root, reading, now)
	}
	start := make(chan struct{})
	results := make(chan *Reservation, len(owners))
	failures := make(chan error, len(owners))
	var joined sync.WaitGroup
	for _, owner := range owners {
		joined.Go(func() {
			<-start
			held, err := owner.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 90}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
			if err != nil && !errors.Is(err, ErrCapacity) {
				failures <- err
			}
			if held != nil {
				results <- held
			}
		})
	}
	close(start)
	joined.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if len(results) != 8 {
		t.Fatalf("got %d successful reservations, want 8", len(results))
	}
	view, err := owners[0].Observe(t.Context())
	if err != nil || view.ReservedBytes != 800 || view.RemainingBytes != 100 {
		t.Fatal("concurrent capacity changed", view, err)
	}
	for held := range results {
		if err := held.Release(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	view, err = owners[0].Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 || view.UsedBytes != 100 {
		t.Fatal("release refunded usage or leaked reserve", view, err)
	}
}
