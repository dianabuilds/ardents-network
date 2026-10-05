package channel

import (
	"sync"
	"testing"
)

func TestChildMemoryRefusalDoesNotConsumeChildSlot(t *testing.T) {
	budget := NewBudget(1)
	for range 1024 {
		if release, err := budget.HoldChild(2); err == nil || release != nil {
			t.Fatal("oversized claim accepted")
		}
	}
	releases := make([]func(), 0, 1024)
	defer func() {
		for _, release := range releases {
			release()
		}
	}()
	for range 1024 {
		release, err := budget.HoldChild(0)
		if err != nil {
			t.Fatal("failed memory claim consumed a slot", err)
		}
		releases = append(releases, release)
	}
	if release, err := budget.HoldChild(0); err == nil || release != nil {
		t.Fatal("child ceiling exceeded")
	}
	releases[0]()
	replacement, err := budget.HoldChild(1)
	if err != nil {
		t.Fatal("released slot unavailable", err)
	}
	defer replacement()
	// A stale release cannot return the replacement's slot or memory.
	releases[0]()
	if release, err := budget.HoldChild(0); err == nil || release != nil {
		t.Fatal("stale release returned a live replacement slot")
	}
}

func TestChildReleaseReturnsSharedMemoryExactlyOnce(t *testing.T) {
	budget := NewBudget(16 << 10)
	child, err := budget.HoldChild(16 << 10)
	if err != nil {
		t.Fatal(err)
	}
	if release, err := budget.HoldControl(); err == nil || release != nil {
		t.Fatal("control bypassed child's principal memory")
	}
	var joined sync.WaitGroup
	for range 32 {
		joined.Go(child)
	}
	joined.Wait()
	control, err := budget.HoldControl()
	if err != nil {
		t.Fatal("joined child memory unavailable", err)
	}
	defer control()
	child()
	if release, err := budget.HoldChild(1); err == nil || release != nil {
		t.Fatal("stale child release erased live control memory")
	}
}
