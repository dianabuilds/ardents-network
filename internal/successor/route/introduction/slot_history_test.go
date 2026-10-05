package introduction

import (
	"errors"
	"testing"
	"time"
)

// This failure-only store cannot acknowledge a retained root or commit bytes.
// It isolates core refusal/terminal retention, not persistence or registration
// success. Native accepting and uncertain-write controls remain on real files.
type refusedHistoryStorage struct {
	verifyErr, closeErr       error
	verifies, commits, closes int
}

func (store *refusedHistoryStorage) verify([]byte) error {
	store.verifies++
	return store.verifyErr
}

func (store *refusedHistoryStorage) commit([]byte) error {
	store.commits++
	return errors.New("failure-only history fixture cannot commit")
}

func (store *refusedHistoryStorage) close() error {
	store.closes++
	return store.closeErr
}

func TestHistoryStorageFailureStopsClaimsAndRetainsOriginalClose(t *testing.T) {
	verifyErr := errors.New("original retained root unavailable")
	closeErr := errors.New("original physical lease release failed")
	store := &refusedHistoryStorage{verifyErr: verifyErr, closeErr: closeErr}
	history := &History{slotSnapshot: slotSnapshot{entries: make(map[[32]byte]time.Time)}, store: store}
	registry, err := NewRegistry(history)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	if capacity, err := registry.Reserve(now); capacity != nil || !errors.Is(err, verifyErr) {
		t.Fatal("unavailable retained storage acquired pending capacity", err)
	}
	if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); !errors.Is(err, verifyErr) {
		t.Fatal("retained storage failure disappeared or permitted a claim", err)
	}
	if capacity, err := registry.Reserve(now); capacity != nil || err == nil {
		t.Fatal("terminal History permitted another capacity attempt")
	}
	first := registry.Close()
	if !errors.Is(first, verifyErr) || !errors.Is(first, closeErr) || registry.Close() != first || history.Close() != first {
		t.Fatal("joined Close lost original failure or repeated result", first)
	}
	if store.verifies != 1 || store.commits != 0 || store.closes != 1 {
		t.Fatal("terminal storage was reused or released more than once", store)
	}
}

func TestRegistryUninitializedHistoryCannotPublishCapacity(t *testing.T) {
	history := &History{}
	if registry, err := NewRegistry(history); registry != nil || err == nil {
		t.Fatal("History without native retained storage acquired a Registry")
	}
	now := time.Unix(1_800_000_000, 0).UTC()
	if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err == nil {
		t.Fatal("uninitialized History acknowledged a claim")
	}
	first := history.Close()
	if first == nil || history.Close() != first {
		t.Fatal("missing physical storage became successful cleanup")
	}
}
