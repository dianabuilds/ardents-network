package instance

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

// waitQueued blocks until exactly n callers are waiting in the coalescing queue,
// proving they were admitted while the current physical round holds its native
// read. The binding's local fence is real-time credential validity, so the oracle
// uses real channels rather than a fake-time bubble.
func waitQueued(t *testing.T, binding *Binding, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		binding.observeMu.Lock()
		got := len(binding.observeQueue)
		binding.observeMu.Unlock()
		if got == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("queued callers = %d, want %d", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestBindingObservationCoalescesFreshRounds proves the rework §4.1 single-flight
// contract for the publication physical observation (audit DD-1): callers admitted
// before a round's native read share that one fresh round; a caller arriving during
// I/O waits for the next complete round; no positive result is retained across
// rounds. The held physical callback controls completion only — it is not a
// successful native custody observation. Mirrors the installation
// startup_check_observation freshness oracle.
func TestBindingObservationCoalescesFreshRounds(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		_ = root.Close()
		_ = history.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = binding.Close(); _ = root.Close(); _ = history.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan int, 3)
	releases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	rounds := 0
	binding.observePhysical = func(context.Context, rootLease) error {
		rounds++
		started <- rounds
		<-releases[rounds-1]
		return nil
	}
	check := func() <-chan error {
		done := make(chan error, 1)
		go func() { done <- binding.observe(ctx) }()
		return done
	}
	first := check()
	if got := <-started; got != 1 {
		t.Fatal("first original round", got)
	}
	// These callers are admitted while round 1 holds its native read; they must
	// share the next fresh round, not borrow round 1's result.
	second, third := check(), check()
	waitQueued(t, binding, 2)
	close(releases[0])
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if got := <-started; got != 2 {
		t.Fatal("queued callers borrowed an earlier round", got)
	}
	for _, done := range []<-chan error{second, third} {
		select {
		case err := <-done:
			t.Fatal("queued caller escaped the held fresh round", err)
		default:
		}
	}
	// A caller admitted during round 2's I/O needs round 3.
	fourth := check()
	waitQueued(t, binding, 1)
	close(releases[1])
	for _, done := range []<-chan error{second, third} {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if got := <-started; got != 3 {
		t.Fatal("caller admitted during I/O reused its result", got)
	}
	select {
	case err := <-fourth:
		t.Fatal("third round did not retain its waiter", err)
	default:
	}
	close(releases[2])
	if err := <-fourth; err != nil || rounds != 3 {
		t.Fatal("complete fresh rounds did not finish", rounds, err)
	}
	if got := binding.observeRoundCount.Load(); got != 3 {
		t.Fatalf("physical round count = %d, want 3", got)
	}
}

// TestBindingObservationCoalescingPreservesRevocation proves coalescing does not
// weaken the local gates: a revoked binding refuses every caller, and a canceled
// caller is refused even when it shares a round with a live caller.
func TestBindingObservationCoalescingPreservesRevocation(t *testing.T) {
	root, config := acceptedNativeRoot(t)
	history, err := durable.Open(context.Background(), config)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	binding, err := root.Bind(context.Background(), history)
	if err != nil {
		_ = root.Close()
		_ = history.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = binding.Close(); _ = root.Close(); _ = history.Close() })

	// A canceled caller shares the batch with a live caller but must still be
	// refused by its own cancellation gate.
	liveCtx, liveCancel := context.WithCancel(context.Background())
	defer liveCancel()
	deadCtx, deadCancel := context.WithCancel(context.Background())
	deadCancel()
	physical := make(chan struct{})
	binding.observePhysical = func(context.Context, rootLease) error {
		<-physical
		return nil
	}
	liveDone := make(chan error, 1)
	deadDone := make(chan error, 1)
	go func() { liveDone <- binding.observe(liveCtx) }()
	go func() { deadDone <- binding.observe(deadCtx) }()
	close(physical)
	if err := <-liveDone; err != nil {
		t.Fatal("live caller refused", err)
	}
	if err := <-deadDone; err == nil {
		t.Fatal("canceled caller accepted through a shared round")
	}

	// After revoke, every caller is refused at the local gate.
	if err := binding.Close(); err != nil {
		t.Fatal(err)
	}
	if err := binding.observe(context.Background()); err == nil {
		t.Fatal("revoked binding accepted an observation")
	}
}
