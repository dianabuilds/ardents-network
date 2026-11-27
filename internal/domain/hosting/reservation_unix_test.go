//go:build linux || darwin

package hosting

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting/counters"
)

func TestCopiedReservationCannotRefundAnotherHandle(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	first, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Rx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 50}, Termination: Traffic{Rx: 10}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	copyOfFirst := *first
	if err = first.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = copyOfFirst.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 60 {
		t.Fatalf("copy refunded other: %+v %v", view, err)
	}
	if err = other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestDetachedTerminationRetainsOriginalCapacityUntilItsOwnReturn(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	ctx := t.Context()
	end := now.Add(time.Minute)
	held, err := b.Reserve(ctx, ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Rx: 20}, WorkUntil: end, HoldUntil: end})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := held.DetachTermination()
	if err != nil {
		t.Fatal(err)
	}
	check := func(want uint64) {
		t.Helper()
		view, err := b.Observe(ctx)
		if err != nil || view.ReservedBytes != want {
			t.Fatalf("reserved %d, want %d: %v", view.ReservedBytes, want, err)
		}
	}
	check(120)
	if _, err := held.DetachTermination(); !errors.Is(err, ErrUnavailable) {
		t.Fatal("original share detached twice", err)
	}
	if _, err := b.ReserveAdditionalJoint(ctx, held, JointTraffic{Tx: 1, Total: 1}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("detached work lent termination coverage", err)
	}
	copyOfWork := *held
	if err := errors.Join(held.Release(ctx), copyOfWork.Release(ctx)); err != nil {
		t.Fatal(err)
	}
	check(20)
	if err := errors.Join(terminal(ctx), terminal(ctx)); err != nil {
		t.Fatal(err)
	}
	check(0)
}

func TestReserveBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name           string
		work, terminal Traffic
		delta          time.Duration
		ok             bool
	}{
		{"exact-watermark", Traffic{Tx: 700}, Traffic{Rx: 100}, time.Minute, true},
		{"watermark-crossed", Traffic{Tx: 701}, Traffic{Rx: 100}, time.Minute, false},
		{"zero-work", Traffic{}, Traffic{Tx: 1}, time.Minute, false},
		{"zero-terminal", Traffic{Tx: 1}, Traffic{}, time.Minute, false},
		{"expired", Traffic{Tx: 1}, Traffic{Rx: 1}, 0, false},
		{"period-end", Traffic{Tx: 1}, Traffic{Rx: 1}, time.Hour, true},
		{"past-period", Traffic{Tx: 1}, Traffic{Rx: 1}, time.Hour + time.Nanosecond, false},
		{"sum-overflow", Traffic{Tx: math.MaxUint64}, Traffic{Rx: 1}, time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, reading, now := hostingFixture(t)
			b := openHostingFixture(t, root, reading, now)
			held, err := b.Reserve(t.Context(), ReservationRequest{Work: tc.work, Termination: tc.terminal, WorkUntil: now.Add(tc.delta), HoldUntil: now.Add(tc.delta)})
			if (err == nil) != tc.ok {
				t.Fatalf("reserve: %v", err)
			}
			if held != nil {
				if err = held.Release(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			view, err := b.Observe(t.Context())
			if err != nil || view.ReservedBytes != 0 {
				t.Fatalf("refusal changed state: %+v %v", view, err)
			}
		})
	}
}

func TestReserveUsesProviderDirections(t *testing.T) {
	for _, tc := range []struct {
		direction string
		cost      uint64
	}{{"tx", 110}, {"rx", 220}, {"tx+rx", 330}} {
		t.Run(tc.direction, func(t *testing.T) {
			now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			reading := counters.Reading{Boot: "fixture-boot", Interfaces: []counters.Interface{{Name: "lo", Index: 1}}}
			policy := Policy{Provider: "fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Unit: "B", Quantity: 1000, Directions: tc.direction, Interfaces: []string{"lo"}, LowWatermarkBytes: 100}
			root := filepath.Join(t.TempDir(), "budget")
			if err := initializeHosting(root, policy, reading, now); err != nil {
				t.Fatal(err)
			}
			b, err := openHosting(root, func([]string) (counters.Reading, error) { return reading, nil }, func() time.Time { return now })
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			held, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100, Rx: 200}, Termination: Traffic{Tx: 10, Rx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(time.Minute)})
			if err != nil {
				t.Fatal(err)
			}
			view, err := b.Observe(t.Context())
			if err != nil || view.ReservedBytes != tc.cost {
				t.Fatalf("direction lost: %+v %v", view, err)
			}
			if err = held.Release(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMeasurementCannotTransferExpiredReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	deadline := now.Add(time.Second)
	owner.measure = func([]string) (counters.Reading, error) {
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
