//go:build linux

package reachability_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Different valid Credentials at one generation model Authority equivocation.
// The Store must retain the longest observed signed lifetime, including further
// conflicts after the Target is already unavailable and after reopening.
func TestPrivateStoreConflictPreservesLongestCredentialExpiry(t *testing.T) {
	for _, restart := range []bool{false, true} {
		for _, expiries := range [][]int{{120}, {120, 180, 90}} {
			t.Run(fmt.Sprintf("restart=%t/conflicts=%d", restart, len(expiries)), func(t *testing.T) {
				fixture := newStoreFixture(t)
				origin := fixture.now
				root, profile := t.TempDir(), [32]byte{71}
				open := func() *reachability.Store {
					store, err := reachability.OpenStore(reachability.StoreConfig{Root: root, NetworkID: fixture.network})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := store.Close(); err != nil {
							t.Error(err)
						}
					})
					return store
				}
				store := open()
				issue := func(generation uint64, start, end time.Time, slot byte) []byte {
					candidate := fixture
					candidate.now = start
					candidate.credential.Generation = generation
					candidate.credential.NotBefore = start.Unix()
					candidate.credential.NotAfter = end.Unix()
					var err error
					candidate.credential, err = candidate.credential.Issue(candidate.authorityKey)
					if err != nil {
						t.Fatal(err)
					}
					current := candidate.publish(t, fmt.Sprintf("publication-%d-%d", generation, slot))
					raw := privateStoreDescriptor(t, candidate, current, 1, slot, start.Add(30*time.Second))
					// A refusal below must arise from Store currentness, not invalid proof.
					if _, err := reachability.VerifyPrivate(raw, current.Credential.Target, fixture.network, profile, start); err != nil {
						t.Fatal(err)
					}
					return raw
				}
				first := issue(1, origin, origin.Add(60*time.Second), 1)
				if result, err := store.PublishPrivate(first, profile, origin); err != nil || result.Class != reachability.StoreAccepted {
					t.Fatalf("first publication: %+v %v", result, err)
				}
				longest := 60
				for i, seconds := range expiries {
					raw := issue(1, origin, origin.Add(time.Duration(seconds)*time.Second), byte(i+2))
					if result, err := store.PublishPrivate(raw, profile, origin); err == nil || (result.Class != reachability.StoreConflicting && result.Class != reachability.StoreStale) {
						t.Fatalf("conflicting publication %d: %+v %v", i, result, err)
					}
					if seconds > longest {
						longest = seconds
					}
					if restart {
						if err := store.Close(); err != nil {
							t.Fatal(err)
						}
						store = open()
					}
				}
				boundary := origin.Add(time.Duration(longest) * time.Second)
				overlappingAt := boundary.Add(-time.Second)
				overlapping := issue(2, overlappingAt, boundary.Add(time.Minute), 10)
				if result, err := store.PublishPrivate(overlapping, profile, overlappingAt); err == nil || result.Class != reachability.StoreInvalid {
					t.Fatalf("generation overlapping observed conflict was accepted: %+v %v", result, err)
				}
				if _, class, err := store.LookupPrivate(fixture.current.Credential.Target, profile, overlappingAt); err == nil || class != reachability.StoreConflicting {
					t.Fatalf("overlap refusal erased conflict: %s %v", class, err)
				}
				later := issue(2, boundary, boundary.Add(time.Minute), 11)
				if result, err := store.PublishPrivate(later, profile, boundary); err != nil || result.Class != reachability.StoreAccepted {
					t.Fatalf("exact non-overlap boundary refused: %+v %v", result, err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				store = open()
				if raw, _, err := store.LookupPrivate(fixture.current.Credential.Target, profile, boundary); err != nil || !bytes.Equal(raw, later) {
					t.Fatalf("reopened repaired generation: %v", err)
				}
			})
		}
	}
}
