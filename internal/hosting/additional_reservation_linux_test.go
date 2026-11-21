//go:build linux

package hosting

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdditionalReservationKeepsOriginalTermination(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 300}, Termination: Traffic{Tx: 30}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	addition, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 200, Total: 200})
	if err != nil {
		t.Fatal(err)
	}
	copyOriginal := *original
	if err := copyOriginal.Release(t.Context()); err == nil {
		t.Fatal("original release claimed completion while addition still borrows termination")
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 530 {
		t.Fatalf("early release returned retained capacity: %+v / %v", view, err)
	}
	if extra, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 1, Total: 1}); err == nil {
		_ = extra.Release(t.Context())
		t.Fatal("sealed original admitted another addition")
	}
	copyAddition := *addition
	if err := addition.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := copyAddition.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err = b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 330 {
		t.Fatalf("addition returned original capacity: %+v / %v", view, err)
	}
	if err := original.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err = b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 0 {
		t.Fatalf("joined release retained capacity: %+v / %v", view, err)
	}
}

func TestAdditionalJointReservationCountsSharedWorkOnce(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100, Rx: 100}, Termination: Traffic{Tx: 20, Rx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	before, err := b.Observe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	addition, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 50, Rx: 50, Total: 50})
	if err != nil {
		t.Fatal(err)
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != before.ReservedBytes+50 {
		t.Fatalf("joint reserve charged twice: before=%+v after=%+v / %v", before, view, err)
	}
	if err = addition.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	view, err = b.Observe(t.Context())
	if err != nil || view.ReservedBytes != before.ReservedBytes {
		t.Fatalf("joint return changed original: %+v / %v", view, err)
	}
	if err = original.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestJointCoverageCannotBorrowTerminationOrForeignOwner(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 200}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	defer original.Release(t.Context())
	if err = b.CoversJoint(t.Context(), original, JointTraffic{Tx: 150, Rx: 150, Total: 150}); !errors.Is(err, ErrCapacity) {
		t.Fatal("termination funded new work", err)
	}
	if err = b.CoversJoint(t.Context(), original, JointTraffic{Tx: 100, Rx: 100, Total: 100}); err != nil {
		t.Fatal(err)
	}
	other := openHostingFixture(t, root, reading, now)
	if err = other.CoversJoint(t.Context(), original, JointTraffic{Tx: 100, Rx: 100, Total: 100}); !errors.Is(err, ErrUnavailable) {
		t.Fatal("foreign budget used original", err)
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 300 {
		t.Fatalf("coverage check changed reservation: %+v / %v", view, err)
	}
}

func TestConcurrentAdditionalReservationsReturnOnlyOwnCapacity(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		held *Reservation
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			h, e := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 50, Total: 50})
			results <- result{h, e}
		}()
	}
	close(start)
	a, c := <-results, <-results
	if a.err != nil || c.err != nil {
		t.Fatalf("concurrent additions: %v / %v", a.err, c.err)
	}
	v, err := b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 220 {
		t.Fatalf("double/omitted reserve: %+v / %v", v, err)
	}
	if err = a.held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	v, err = b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 170 {
		t.Fatalf("returned sibling: %+v / %v", v, err)
	}
	if err = c.held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = original.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestAdditionalPostCommitCancellationReturnsOnlyAddition(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	b.now = func() time.Time {
		state, e := readCommittedHostingState(b.root)
		if e != nil {
			t.Fatal(e)
		}
		if state.Reserved == 170 {
			cancel()
		}
		return *now
	}
	addition, err := b.ReserveAdditionalJoint(ctx, original, JointTraffic{Tx: 50, Total: 50})
	if addition != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation transferred: %v / %v", addition, err)
	}
	v, err := b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 120 {
		t.Fatalf("cleanup returned original: %+v / %v", v, err)
	}
	if err = original.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestAdditionalPostCommitOriginalReleasePreventsHandoff(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	sealed := false
	b.now = func() time.Time {
		state, e := readCommittedHostingState(b.root)
		if e != nil {
			t.Fatal(e)
		}
		// Only the handoff clock runs outside the budget transaction. Releasing
		// from an in-transaction clock would reenter its gate, not model a caller.
		if !sealed && state.Reserved == 170 && len(b.gate) != 0 {
			sealed = true
			if e = original.Release(t.Context()); !errors.Is(e, ErrReservationInUse) {
				t.Fatalf("early original release: %v", e)
			}
		}
		return *now
	}
	addition, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 50, Total: 50})
	if addition != nil || !errors.Is(err, ErrCapacity) || !sealed {
		t.Fatalf("sealed original transferred addition: %v / %v / %v", addition, err, sealed)
	}
	v, err := b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 120 {
		t.Fatalf("late cleanup returned original coverage: %+v / %v", v, err)
	}
	if err = original.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	v, err = b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 0 {
		t.Fatalf("joined original retained capacity: %+v / %v", v, err)
	}
}

func TestAdditionalUncertainReturnRetainsOriginalCoverage(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	addition, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 50, Total: 50})
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "budget.json")
	saved := state + ".saved"
	fault := true
	b.measure = func([]string) (hostingReading, error) {
		if fault {
			fault = false
			if e := os.Rename(state, saved); e != nil {
				return hostingReading{}, e
			}
			if e := os.Mkdir(state, 0700); e != nil {
				return hostingReading{}, e
			}
		}
		return *reading, nil
	}
	first := addition.Release(t.Context())
	if first == nil {
		t.Fatal("uncertain refund reported success")
	}
	if err = os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(saved, state); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "budget.pending")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err = addition.Release(t.Context()); err != first {
		t.Fatalf("uncertain return retried: %v / %v", err, first)
	}
	if err = original.Release(t.Context()); !errors.Is(err, first) {
		t.Fatalf("original lost late failure: %v", err)
	}
	v, err := b.Observe(t.Context())
	if err != nil || v.ReservedBytes != 170 {
		t.Fatalf("uncertainty released coverage: %+v / %v", v, err)
	}
}

func TestAdditionalReservationRejectsForeignAndExpiredOriginal(t *testing.T) {
	root, reading, now := hostingFixture(t)
	b := openHostingFixture(t, root, reading, now)
	other := openHostingFixture(t, root, reading, now)
	original, err := b.Reserve(t.Context(), ReservationRequest{Work: Traffic{Tx: 100}, Termination: Traffic{Tx: 20}, WorkUntil: now.Add(time.Minute), HoldUntil: now.Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	defer original.Release(t.Context())
	if addition, err := other.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 10, Total: 10}); err == nil {
		_ = addition.Release(t.Context())
		t.Fatal("another owner acquired original coverage")
	}
	*now = now.Add(time.Minute)
	if addition, err := b.ReserveAdditionalJoint(t.Context(), original, JointTraffic{Tx: 10, Total: 10}); err == nil {
		_ = addition.Release(t.Context())
		t.Fatal("addition renewed original work deadline")
	}
	view, err := b.Observe(t.Context())
	if err != nil || view.ReservedBytes != 120 {
		t.Fatalf("refusal changed retained capacity: %+v / %v", view, err)
	}
}
