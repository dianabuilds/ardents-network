//go:build linux && document_worker_installed

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/publication"
	"github.com/dianabuilds/ardents-network/internal/reachability"
)

// InstalledPairStoredProof verifies the authenticated receiver Store entry for an installed Link.
func InstalledPairStoredProof(t *testing.T, store *reachability.Store, networkID, profileDigest [32]byte, link string) reachability.Descriptor {
	t.Helper()
	network, target, err := publication.ParseTargetLink(link)
	if err != nil || network != networkID {
		t.Fatal("installed Link lacks original Network", err)
	}
	return installedStoredProof(t, store, network, profileDigest, target)
}

func installedStoredProof(t *testing.T, store *reachability.Store, network, profileDigest, target [32]byte) reachability.Descriptor {
	t.Helper()
	now := time.Now()
	raw, outcome, err := store.Lookup(target, profileDigest, now)
	if err != nil || outcome != reachability.Accepted {
		t.Fatal("installed publication lacks actual accepted Store proof", outcome, err)
	}
	proof, err := reachability.Verify(raw, target, network, profileDigest, now)
	if err != nil {
		t.Fatal("installed Store proof failed independent verification", err)
	}
	return proof
}

// InstalledPairAwaitRefresh observes the Store until its automatic refresh is acknowledged.
func InstalledPairAwaitRefresh(t *testing.T, store *reachability.Store, networkID, profileDigest [32]byte, link string, first reachability.Descriptor, publishStarted time.Time) {
	t.Helper()
	// No forced Refresh, clock adjustment or shortened product scheduler. This
	// observer alone polls the actual receiving Store under the timing profile.
	deadline := time.Now().Add(6 * time.Minute)
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		proof := InstalledPairStoredProof(t, store, networkID, profileDigest, link)
		if proof.Introduction.Revision > first.Introduction.Revision {
			// Registration cannot predate this actual Snapshot request. Its
			// canonical whole-second creation may round down by less than 1s.
			if time.Now().Before(publishStarted.Add(299 * time.Second)) {
				t.Fatal("installed refresh preceded original creation+300s lower bound")
			}
			requireFreshDescriptor(t, first, proof)
			t.Logf("actual installed Store acknowledged automatic revision %d -> %d", first.Introduction.Revision, proof.Introduction.Revision)
			return
		}
		if proof.Digest() != first.Digest() {
			t.Fatal("installed Store changed original revision bytes")
		}
		if !time.Now().Before(deadline) {
			t.Fatal("installed automatic refresh lacked higher acknowledged revision")
		}
		select {
		case <-t.Context().Done():
			t.Fatal("installed refresh observation interrupted", t.Context().Err())
		case <-tick.C:
		}
	}
}

// VerifyInstalledDocumentRefresh checks that the new descriptor takes over while the original proof is still current.
func VerifyInstalledDocumentRefresh(t *testing.T, ctx context.Context, store *reachability.Store, network, profileDigest, target [32]byte, requireRetired func(reachability.Descriptor)) {
	t.Helper()
	first := installedStoredProof(t, store, network, profileDigest, target)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("automatic Publication refresh missing", ctx.Err())
		case <-ticker.C:
		}
		next := installedStoredProof(t, store, network, profileDigest, target)
		if next.Introduction.Revision == first.Introduction.Revision {
			continue
		}
		requireFreshDescriptor(t, first, next)

		// Store commit precedes ACK. Allow overlap to finish before probing the original recipient.
		overlap := time.NewTimer(62 * time.Second)
		defer overlap.Stop()
		select {
		case <-overlap.C:
		case <-ctx.Done():
			t.Fatal("original predecessor did not reach the retirement boundary", ctx.Err())
		}
		if err := first.Current(time.Now()); err != nil {
			t.Fatal("original signed Descriptor expired before predecessor refusal", err)
		}
		requireRetired(first)
		current := installedStoredProof(t, store, network, profileDigest, target)
		if current.Introduction.Revision != next.Introduction.Revision || current.Introduction.Slot != next.Introduction.Slot {
			t.Fatal("retiring the predecessor changed the accepting replacement")
		}
		return
	}
}

func requireFreshDescriptor(t *testing.T, first, next reachability.Descriptor) {
	t.Helper()
	if next.Introduction.Revision != first.Introduction.Revision+1 || next.PublicationDigest != first.PublicationDigest ||
		next.Introduction.Slot == first.Introduction.Slot || next.Introduction.RecipientKey == first.Introduction.RecipientKey {
		t.Fatal("automatic refresh changed Publication or failed to advance to the next revision with a fresh recipient")
	}
}
