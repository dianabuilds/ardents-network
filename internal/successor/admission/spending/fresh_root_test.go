package spending

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestFreshRootFactIsOneUseAndNeverReissuedOnRetainedOpen(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	ledger, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := ledger.TakeFreshRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.TakeFreshRoot(); err == nil {
		t.Fatal("issued freshness twice")
	}
	initialization, err := fact.Begin(binding)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fact.Begin(binding); err == nil {
		t.Fatal("consumed freshness twice")
	}
	if err := initialization.Check(); err != nil {
		t.Fatal(err)
	}
	if err := initialization.Complete(); err != nil {
		t.Fatal(err)
	}
	if err := initialization.Check(); err == nil {
		t.Fatal("completed initialization remained usable")
	}
	if err := initialization.Complete(); err == nil {
		t.Fatal("completed initialization twice")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.TakeFreshRoot(); err == nil {
		t.Fatal("header-only retained journal authorized new companion history")
	}
}

func TestPrunedEmptyJournalCannotMintFreshRootFact(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	ledger, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	token := make([]byte, 354)
	token[0] = 1
	if err := ledger.Spend(token, window, window.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := ledger.prune(window.Add(time.Hour + time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(ledger.spent) != 0 {
		t.Fatal("expected actual empty compaction")
	}
	if _, err := ledger.TakeFreshRoot(); err == nil {
		t.Fatal("compaction reissued freshness")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.TakeFreshRoot(); err == nil {
		t.Fatal("pruned retained root reissued freshness")
	}
}

func TestConcurrentFreshRootTakeAndCompletionAreExactlyOnce(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := Open(t.TempDir(), binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	start := make(chan struct{})
	facts := make(chan *FreshRoot, 16)
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			<-start
			fact, err := ledger.TakeFreshRoot()
			if err == nil {
				facts <- fact
			}
		})
	}
	close(start)
	workers.Wait()
	close(facts)
	if len(facts) != 1 {
		t.Fatalf("issued %d concurrent facts", len(facts))
	}
	initialization, err := (<-facts).Begin(binding)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 16)
	for range 16 {
		workers.Go(func() { results <- initialization.Complete() })
	}
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("completed %d concurrent handoffs", successes)
	}
}

func TestFreshRootFactRefusesInvalidationAtEveryHandoff(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	for _, stage := range []string{"before take", "before begin", "before completion"} {
		for _, cause := range []string{"close", "admission", "malformed spend"} {
			t.Run(stage+"/"+cause, func(t *testing.T) {
				ledger, err := Open(t.TempDir(), binding)
				if err != nil {
					t.Fatal(err)
				}
				defer ledger.Close()
				invalidate := func() {
					switch cause {
					case "close":
						if err := ledger.Close(); err != nil {
							t.Fatal(err)
						}
					case "admission":
						ledger.InvalidateFreshRoot()
					case "malformed spend":
						if err := ledger.Spend(nil, time.Time{}, time.Time{}); err == nil {
							t.Fatal("invalid spend succeeded")
						}
					}
				}
				if stage == "before take" {
					invalidate()
					if _, err := ledger.TakeFreshRoot(); err == nil {
						t.Fatal("invalid owner issued fact")
					}
					return
				}
				fact, err := ledger.TakeFreshRoot()
				if err != nil {
					t.Fatal(err)
				}
				if stage == "before begin" {
					invalidate()
					if _, err := fact.Begin(binding); err == nil {
						t.Fatal("stale fact began initialization")
					}
					return
				}
				initialization, err := fact.Begin(binding)
				if err != nil {
					t.Fatal(err)
				}
				invalidate()
				if err := initialization.Check(); err == nil {
					t.Fatal("stale initialization authorized effect")
				}
				if err := initialization.Complete(); err == nil {
					t.Fatal("late initialization published success")
				}
			})
		}
	}
}

func TestFreshRootFactRejectsForeignBindingAndZeroValues(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := Open(t.TempDir(), binding)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	fact, err := ledger.TakeFreshRoot()
	if err != nil {
		t.Fatal(err)
	}
	foreign := binding
	foreign.ReceiverDutyGeneration++
	if _, err := fact.Begin(foreign); err == nil {
		t.Fatal("authorized foreign duty")
	}
	if _, err := fact.Begin(binding); err == nil {
		t.Fatal("failed consumption retried")
	}
	var zero FreshRoot
	if _, err := zero.Begin(binding); err == nil {
		t.Fatal("zero fact authorized initialization")
	}
	var initialization RootInitialization
	if err := initialization.Check(); err == nil {
		t.Fatal("zero initialization authorized effect")
	}
	if err := initialization.Complete(); err == nil {
		t.Fatal("zero initialization authorized completion")
	}
}

func TestFailedFreshDurabilityCannotIssueFactOnReopen(t *testing.T) {
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	root := t.TempDir()
	failure := errors.New("initial durability unavailable")
	ledger, err := openLedger(root, binding, func(string) error { return failure })
	if ledger != nil || !errors.Is(err, failure) {
		t.Fatalf("uncertain initialization = %v / %v", ledger, err)
	}
	reopened, err := Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.TakeFreshRoot(); err == nil {
		t.Fatal("reopened uncertain creation minted freshness")
	}
}
