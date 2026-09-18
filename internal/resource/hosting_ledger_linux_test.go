//go:build linux

package resource

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func hostingFixture(t *testing.T) (string, *hostingReading, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	reading := &hostingReading{Boot: "boot-one", Interfaces: []hostingInterface{{Name: "eth0", Index: 2, Tx: 500, Rx: 800}}}
	root := filepath.Join(t.TempDir(), "hosting")
	policy := HostingPolicy{Provider: "declared fixture provider", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "B", Quantity: 1000, Directions: "tx+rx", Interfaces: []string{"eth0"}, InitialUsedBytes: 100, LowWatermarkBytes: 100}
	if err := initializeHosting(root, policy, *reading, now); err != nil {
		t.Fatal(err)
	}
	return root, reading, &now
}

func openHostingFixture(t *testing.T, root string, reading *hostingReading, now *time.Time) *Hosting {
	t.Helper()
	owner, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	return owner
}

func TestHostingReservationsShareOneDurablePeriod(t *testing.T) {
	root, reading, now := hostingFixture(t)
	first := openHostingFixture(t, root, reading, now)
	second := openHostingFixture(t, root, reading, now)
	held, err := first.Reserve(t.Context(), HostingTraffic{Tx: 100, Rx: 200}, HostingTraffic{Tx: 10, Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Reserve(t.Context(), HostingTraffic{Tx: 600}, HostingTraffic{Rx: 100}, now.Add(time.Minute)); err == nil {
		t.Fatal("another owner multiplied the host allowance")
	}
	other, err := second.Reserve(t.Context(), HostingTraffic{Tx: 220}, HostingTraffic{Rx: 30}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	reading.Interfaces[0].Tx += 40
	reading.Interfaces[0].Rx += 60
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	got, err := second.Observe(t.Context())
	if err != nil || got.UsedBytes != 200 || got.ReservedBytes != 250 || got.RemainingBytes != 550 {
		t.Fatalf("shared accounting or exact release differs: %+v / %v", got, err)
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestHostingSampleSharesOneRecentCommittedObservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	measurements := 0
	measure := func([]string) (hostingReading, error) {
		measurements++
		return *reading, nil
	}
	first, err := openHosting(root, measure, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := openHosting(root, measure, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close() })
	opened := measurements
	reading.Interfaces[0].Tx += 40
	*now = now.Add(500 * time.Millisecond)
	shared, err := first.Sample(t.Context(), time.Second)
	if err != nil || measurements != opened || shared.At != now.Add(-500*time.Millisecond) || shared.Interfaces[0].Tx != reading.Interfaces[0].Tx-40 {
		t.Fatalf("recent committed sample was not shared: %+v measurements=%d opened=%d err=%v", shared, measurements, opened, err)
	}
	*now = now.Add(501 * time.Millisecond)
	refreshed, err := first.Sample(t.Context(), time.Second)
	if err != nil || measurements != opened+1 || refreshed.At != *now || refreshed.Interfaces[0].Tx != reading.Interfaces[0].Tx {
		t.Fatalf("expired sample was not refreshed: %+v measurements=%d opened=%d err=%v", refreshed, measurements, opened, err)
	}
	shared, err = second.Sample(t.Context(), time.Second)
	if err != nil || measurements != opened+1 || shared.At != refreshed.At || shared.Observation != refreshed.Observation {
		t.Fatalf("second owner did not share refreshed sample: %+v measurements=%d err=%v", shared, measurements, err)
	}
}

func TestHostingSampleDoesNotJoinExclusiveLeaseQueueForRecentCommit(t *testing.T) {
	root, reading, now := hostingFixture(t)
	measurements := 0
	owner, err := openHosting(root, func([]string) (hostingReading, error) {
		measurements++
		return *reading, nil
	}, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })

	lock, err := os.OpenFile(filepath.Join(root, "period.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) })
	if err := os.WriteFile(filepath.Join(root, "period.pending"), []byte("in-flight replacement"), 0600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	shared, err := owner.Sample(ctx, time.Second)
	if err != nil || measurements != 0 || shared.At != *now || shared.Observation.UsedBytes != 100 {
		t.Fatalf("recent sample joined the exclusive lease queue: %+v measurements=%d err=%v", shared, measurements, err)
	}
}

func TestHostingSampleWaitsForAnotherWriterWithoutJoiningItsExclusiveQueue(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	*now = now.Add(2 * time.Second)

	lock, err := os.OpenFile(filepath.Join(root, "period.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) })

	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, sampleErr := owner.Sample(ctx, time.Second)
		result <- sampleErr
	}()
	// Let Sample observe the expired commit while this writer owns the lease,
	// then publish the writer's fresh atomic replacement without releasing it.
	time.Sleep(25 * time.Millisecond)
	state, err := readCommittedHostingState(owner.root)
	if err != nil {
		t.Fatal(err)
	}
	state.Observed = *now
	if err := writeHostingState(owner.root, state); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("sample did not share the active writer's fresh commit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("sample joined the active writer's exclusive lease queue")
	}
}

func TestHostingSampleLocalGateHonorsContext(t *testing.T) {
	root, reading, now := hostingFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	block := false
	owner, err := openHosting(root, func([]string) (hostingReading, error) {
		if block {
			close(entered)
			<-release
		}
		return *reading, nil
	}, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	*now = now.Add(2 * time.Second)
	block = true
	first := make(chan error, 1)
	go func() {
		_, sampleErr := owner.Sample(t.Context(), time.Second)
		first <- sampleErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first sample did not enter measurement")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	second := make(chan error, 1)
	go func() {
		_, sampleErr := owner.Sample(ctx, time.Second)
		second <- sampleErr
	}()
	select {
	case err := <-second:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("contending local sample = %v, want context deadline", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("contending local sample ignored its context")
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
}

func TestHostingReopenDoesNotRefundAbandonedWork(t *testing.T) {
	root, reading, now := hostingFixture(t)
	first := openHostingFixture(t, root, reading, now)
	if _, err := first.Reserve(t.Context(), HostingTraffic{Tx: 200}, HostingTraffic{Rx: 100}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := openHostingFixture(t, root, reading, now)
	got, err := reopened.Observe(t.Context())
	if err != nil || got.UsedBytes != 100 || got.ReservedBytes != 300 {
		t.Fatalf("restart refunded work: %+v / %v", got, err)
	}
	*now = now.Add(2 * time.Hour)
	got, err = reopened.Observe(t.Context())
	if err != nil || !got.Drain || got.ReservedBytes != 300 {
		t.Fatalf("period expiry reset accounting: %+v / %v", got, err)
	}
	if _, err := reopened.Reserve(t.Context(), HostingTraffic{Tx: 1}, HostingTraffic{Rx: 1}, now.Add(time.Second)); err == nil {
		t.Fatal("expired period admitted new work")
	}
}

func TestHostingStartsAnAlreadyExhaustedPeriodOnlyToDrain(t *testing.T) {
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	reading := hostingReading{Boot: "boot-one", Interfaces: []hostingInterface{{Name: "eth0", Index: 2, Tx: 500, Rx: 800}}}
	root := filepath.Join(t.TempDir(), "hosting")
	policy := HostingPolicy{Provider: "declared fixture provider", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
		Unit: "B", Quantity: 1000, Directions: "tx+rx", Interfaces: []string{"eth0"}, InitialUsedBytes: 1000, LowWatermarkBytes: 100}
	if err := initializeHosting(root, policy, reading, now); err != nil {
		t.Fatalf("exhausted provider period must retain its drain floor: %v", err)
	}
	owner := openHostingFixture(t, root, &reading, &now)
	got, err := owner.Observe(t.Context())
	if err != nil || !got.Protect || !got.Drain || got.RemainingBytes != 0 {
		t.Fatalf("exhausted provider period must drain without a reset: %+v / %v", got, err)
	}
	if _, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 1}, HostingTraffic{Rx: 1}, now.Add(time.Minute)); err == nil {
		t.Fatal("exhausted provider period admitted new work")
	}
}

func TestHostingLostCounterContinuityRefuses(t *testing.T) {
	for _, change := range []string{"boot", "interface", "tx", "clock"} {
		t.Run(change, func(t *testing.T) {
			root, reading, now := hostingFixture(t)
			owner := openHostingFixture(t, root, reading, now)
			switch change {
			case "boot":
				reading.Boot = "boot-two"
			case "interface":
				reading.Interfaces[0].Index++
			case "tx":
				reading.Interfaces[0].Tx--
			case "clock":
				*now = now.Add(-time.Second)
			}
			got, err := owner.Observe(t.Context())
			if err == nil || !got.Drain {
				t.Fatalf("unknown counter tail admitted: %+v / %v", got, err)
			}
		})
	}
}

func TestHostingCorruptStateIsNeverInitializedAgain(t *testing.T) {
	root, reading, now := hostingFixture(t)
	raw, err := os.ReadFile(filepath.Join(root, "period.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "period.json"), raw[:len(raw)/2], 0600); err != nil {
		t.Fatal(err)
	}
	if owner, err := openHosting(root, func([]string) (hostingReading, error) { return *reading, nil }, func() time.Time { return *now }); err == nil {
		_ = owner.Close()
		t.Fatal("corrupt period reopened")
	}
	if err := initializeHosting(root, HostingPolicy{}, *reading, *now); err == nil {
		t.Fatal("existing corrupt period was reset")
	}
}

func TestHostingCancellationPreventsReservation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := owner.Reserve(canceled, HostingTraffic{Tx: 100}, HostingTraffic{Rx: 20}, now.Add(time.Minute)); err == nil {
		t.Fatal("canceled work admitted")
	}
	got, err := owner.Observe(t.Context())
	if err != nil || got.ReservedBytes != 0 {
		t.Fatalf("cancellation changed reserve: %+v / %v", got, err)
	}
}

func TestHostingCanceledReleaseRetriesOnlyBeforeMutation(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	first, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 100}, HostingTraffic{Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	other, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 50}, HostingTraffic{Rx: 10}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := first.Release(canceled); err == nil {
		t.Fatal("canceled release mutated the reservation")
	}
	reading.Interfaces[0].Tx += 40
	start := make(chan struct{})
	errs := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			errs <- first.Release(t.Context())
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	for releaseErr := range errs {
		if releaseErr != nil {
			t.Fatalf("live retry after pre-mutation cancellation = %v", releaseErr)
		}
	}
	if err := first.Release(t.Context()); err != nil {
		t.Fatalf("repeated release = %v", err)
	}
	got, err := owner.Observe(t.Context())
	if err != nil || got.UsedBytes != 140 || got.ReservedBytes != 60 {
		t.Fatalf("retry changed observed traffic or another reservation: %+v / %v", got, err)
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestHostingReleaseAfterCallbackRefusalStaysUnresolved(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	first, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 100}, HostingTraffic{Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	other, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 50}, HostingTraffic{Rx: 10}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	first.bytes = 1000
	if err := first.Release(t.Context()); err == nil {
		t.Fatal("callback refusal released a reservation")
	}
	if err := first.Release(t.Context()); err == nil {
		t.Fatal("callback refusal retried a potentially committed release")
	}
	got, err := owner.Observe(t.Context())
	if err != nil || got.ReservedBytes != 180 {
		t.Fatalf("callback refusal refunded capacity: %+v / %v", got, err)
	}
	if err := other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestHostingReleaseAfterPersistenceFailureStaysUnresolved(t *testing.T) {
	root, reading, now := hostingFixture(t)
	state := filepath.Join(root, "period.json")
	saved := state + ".saved"
	fault := false
	owner, err := openHosting(root, func([]string) (hostingReading, error) {
		if fault {
			fault = false
			if err := os.Rename(state, saved); err != nil {
				return hostingReading{}, err
			}
			if err := os.Mkdir(state, 0o700); err != nil {
				return hostingReading{}, err
			}
		}
		return *reading, nil
	}, func() time.Time { return *now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	first, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 100}, HostingTraffic{Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	other, err := owner.Reserve(t.Context(), HostingTraffic{Tx: 50}, HostingTraffic{Rx: 10}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	fault = true
	if err = first.Release(t.Context()); !errors.Is(err, syscall.EEXIST) {
		t.Fatalf("post-callback persistence failure = %v, want EEXIST", err)
	}
	firstErr := err
	if !first.released {
		t.Fatal("post-callback persistence failure left the handle retryable")
	}
	if err = os.Remove(state); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(saved, state); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "period.pending")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err = first.Release(t.Context()); err != firstErr {
		t.Fatalf("persistence failure retried after filesystem recovery = %v, want cached %v", err, firstErr)
	}
	got, err := owner.Observe(t.Context())
	if err != nil || got.ReservedBytes != 180 {
		t.Fatalf("persistence failure refunded a reservation: %+v / %v", got, err)
	}
	if err = other.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}
