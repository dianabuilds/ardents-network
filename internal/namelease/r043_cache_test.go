package namelease

import (
	"crypto/rand"
	"testing"
)

// TestR043_CacheFreshOnSameEpoch: a CacheEntry bound to the current
// epoch with the matching digest is fresh.
func TestR043_CacheFreshOnSameEpoch(t *testing.T) {
	t.Parallel()
	var d [32]byte
	_, _ = rand.Read(d[:])
	e := cacheEntry{Name: "c.example", Target: "t1", EpochNumber: 7, EpochDigest: d}
	if !e.isFresh(7, d) {
		t.Errorf("IsFresh: same epoch + digest must be fresh")
	}
	if err := e.checkFresh(7, d); err != nil {
		t.Errorf("CheckFresh: %v", err)
	}
}

// TestR043_CacheStaleOnOlderEpoch: an entry from a previous epoch is
// stale-proof, even with the same digest, because R-043 binds every
// read to the current epoch.
func TestR043_CacheStaleOnOlderEpoch(t *testing.T) {
	t.Parallel()
	var d [32]byte
	_, _ = rand.Read(d[:])
	e := cacheEntry{Name: "c.example", Target: "t1", EpochNumber: 6, EpochDigest: d}
	if e.isFresh(7, d) {
		t.Errorf("IsFresh: old epoch must be stale")
	}
	if err := e.checkFresh(7, d); err == nil {
		t.Errorf("CheckFresh: old epoch must fail")
	}
}

// TestR043_CacheStaleOnMismatchedDigest: an entry from the current
// epoch but with a different digest is stale-proof (fork or replay
// attack across epoch transitions).
func TestR043_CacheStaleOnMismatchedDigest(t *testing.T) {
	t.Parallel()
	var d1, d2 [32]byte
	_, _ = rand.Read(d1[:])
	_, _ = rand.Read(d2[:])
	e := cacheEntry{Name: "c.example", Target: "t1", EpochNumber: 7, EpochDigest: d1}
	if e.isFresh(7, d2) {
		t.Errorf("IsFresh: mismatched digest must be stale")
	}
	if err := e.checkFresh(7, d2); err == nil {
		t.Errorf("CheckFresh: mismatched digest must fail")
	}
}

// TestR043_CacheMissingProof: an entry with no epoch (zero
// EpochNumber) is fail-closed (no silent fallback to unbounded
// freshness).
func TestR043_CacheMissingProof(t *testing.T) {
	t.Parallel()
	var d [32]byte
	_, _ = rand.Read(d[:])
	e := cacheEntry{Name: "c.example", Target: "t1"}
	if err := e.checkFresh(7, d); err == nil {
		t.Errorf("CheckFresh: zero EpochNumber must fail (missing proof)")
	}
}
