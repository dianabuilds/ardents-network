package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

func TestAcceptClosedProfilePersistsAndConflictsByArrival(t *testing.T) {
	store, first := closedProfileStoreFixture(t)
	now := store.config.clock()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network, epochDigest := store.current.Snapshot.NetworkID, store.current.Snapshot.Digest
	candidate := store.current.Candidates[0]
	nodeID := candidate.NodeID
	node := closedProfileNode{nodeID: nodeID, recordDigest: candidate.RecordDigest,
		domain: 2, subrole: 6, generation: candidate.RecordGeneration}
	root := store.storage
	parsed, parseErr := closedprofile.Verify(first, closedprofile.Context{StateGeneration: generation, NetworkID: network, EpochDigest: epochDigest, Epoch: 9, Authority: authority.Public().(ed25519.PublicKey), Now: now})
	_, joinErr := bindMembership(parsed, store.current.Candidates)
	if parseErr != nil || joinErr != nil {
		t.Fatalf("closed profile parser/join = %v, %v", parseErr, joinErr)
	}
	view, err := store.AcceptClosedProfile(first)
	if err != nil || view.Digest != sha256.Sum256(first) {
		t.Fatalf("accept first closed profile = %+v, %v", view, err)
	}
	current, err := store.CurrentRuntime()
	if err != nil || current.Profile().Digest != view.Digest {
		t.Fatalf("current closed profile = %+v, %v", current, err)
	}
	route, err := store.CurrentRuntime()
	if err != nil || route.Profile().Digest != view.Digest || len(route.Members()) != 1 || route.Members()[0].NodeID != nodeID ||
		route.Members()[0].RecordDigest != node.recordDigest || route.Members()[0].RoleDomain != node.domain ||
		route.Members()[0].Subrole != node.subrole || route.Members()[0].DutyGeneration != node.generation {
		t.Fatalf("current closed route = %+v, %v", route, err)
	}
	second := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	if _, err := store.AcceptClosedProfile(second); err == nil {
		t.Fatal("accepted a second closed profile digest")
	}
	if _, err := store.CurrentRuntime(); err == nil {
		t.Fatal("returned a closed profile after durable conflict")
	}
	state, _, err := root.LoadClosedProfile(generation)
	if err != nil || state.Conflict != sha256.Sum256(second) {
		t.Fatalf("durable profile conflict = %+v, %v", state, err)
	}
}

func TestAcceptClosedProfileRetriesInterruptedPublication(t *testing.T) {
	rootPath := t.TempDir()
	store, first := closedProfileStoreFixtureAt(t, rootPath)
	generation := sha256.Sum256([]byte("closed profile generation"))
	profilePath := filepath.Join(rootPath, fmt.Sprintf("closed-profile-%x.bin", generation))
	if err := os.WriteFile(profilePath, first, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CurrentRuntime(); err == nil {
		t.Fatal("published profile bytes without durable state")
	}
	view, err := store.AcceptClosedProfile(first)
	if err != nil || view.Digest != sha256.Sum256(first) {
		t.Fatalf("retry exact profile after interrupted publication: digest=%x, err=%v", view.Digest, err)
	}
	current, err := store.CurrentRuntime()
	if err != nil || current.Profile().Digest != view.Digest {
		t.Fatalf("current closed profile after retry: digest=%x, err=%v", current.Profile().Digest, err)
	}
}

func closedProfileStoreFixture(t *testing.T) (*networkState, []byte) {
	return closedProfileStoreFixtureAt(t, t.TempDir())
}

func closedProfileStoreFixtureAt(t *testing.T, rootPath string) (*networkState, []byte) {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	generation := sha256.Sum256([]byte("closed profile generation"))
	network := sha256.Sum256([]byte("closed profile network"))
	epochDigest := sha256.Sum256([]byte("closed profile epoch"))
	nodeID := sha256.Sum256([]byte("issuer node"))
	recordRaw := []byte("authenticated schema-2 record")
	recordGeneration := uint64(5)
	root, err := openTestDurableRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	store := &networkState{config: config{closedProfileAuthority: authority.Public().(ed25519.PublicKey), clock: func() time.Time { return now }, observe: func() time.Time { return now }}, storage: root,
		current: &epoch.Decision{Snapshot: epoch.Snapshot{Generation: fmt.Sprintf("%x", generation), NetworkID: network, Epoch: 9, Digest: epochDigest,
			EpochValidFrom: now.Truncate(time.Hour), ValidUntil: now.Truncate(time.Hour).Add(2 * time.Hour), Profile: closedRouteProfile},
			Candidates: []epoch.Candidate{{
				NodeID: nodeID, RecordDigest: sha256.Sum256(recordRaw), RecordGeneration: recordGeneration,
				CarrierProfile: closedTCPCarrierProfile, Domain: "rendezvous",
			}}}}
	// This fixture isolates profile persistence after Epoch authentication.
	// Keep the authenticated header coherent with its diagnostic projection.
	store.current.Header = epoch.Header{NetworkID: network, Number: 9, Digest: epochDigest,
		ValidFrom: store.current.Snapshot.EpochValidFrom, ValidUntil: store.current.Snapshot.ValidUntil, Profile: closedRouteProfile}
	node := closedProfileNode{nodeID: nodeID, recordDigest: sha256.Sum256(recordRaw), domain: 2, subrole: 6, generation: recordGeneration}
	first := testClosedProfile(t, authority, network, generation, epochDigest, now, []closedProfileNode{node})
	return store, first
}

func TestAcceptClosedProfileUncertainStateRecordRetiresLiveOwner(t *testing.T) {
	store, first := closedProfileStoreFixture(t)
	canceled := false
	store.workCancel = func() { canceled = true }
	failure := errors.New("injected state-directory sync failure")
	_, err := store.acceptClosedProfileWithCommit(first, func(state durable.ClosedProfileState, raw []byte) error {
		if err := store.storage.CommitClosedProfile(state, raw); err != nil {
			return err
		}
		return fmt.Errorf("%w: %w", durable.ErrClosedProfileStateSyncUncertain, failure)
	})
	if !errors.Is(err, durable.ErrClosedProfileStateSyncUncertain) || !errors.Is(err, failure) || !store.closed || !canceled {
		t.Fatalf("uncertain acceptance: err=%v closed=%t canceled=%t", err, store.closed, canceled)
	}
	generation := sha256.Sum256([]byte("closed profile generation"))
	stored, raw, err := store.storage.LoadClosedProfile(generation)
	if err != nil || stored.Accepted != sha256.Sum256(first) || !bytes.Equal(raw, first) {
		t.Fatalf("visible accepted profile after failure: state=%+v bytes=%d err=%v", stored, len(raw), err)
	}
	if value, err := store.CurrentRuntime(); err == nil || value.Check(time.Now()) == nil {
		t.Fatalf("retired State exposed profile: %+v, %v", value, err)
	}
	if value, err := store.CurrentRuntime(); err == nil || value.Check(time.Now()) == nil {
		t.Fatalf("retired State exposed route: %+v, %v", value, err)
	}
	if err := store.Wait(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Wait lost terminal cause: %v", err)
	}
	if err := store.Close(); !errors.Is(err, failure) {
		t.Fatalf("Close lost terminal cause: %v", err)
	}
}

func TestAcceptClosedProfilePreRenameFailureAllowsExactRetry(t *testing.T) {
	store, first := closedProfileStoreFixture(t)
	failure := errors.New("injected pre-rename failure")
	if _, err := store.acceptClosedProfileWithCommit(first, func(durable.ClosedProfileState, []byte) error {
		return failure
	}); !errors.Is(err, failure) || store.closed {
		t.Fatalf("pre-rename failure: err=%v closed=%t", err, store.closed)
	}
	if _, err := store.CurrentRuntime(); err == nil {
		t.Fatal("served a profile without a durable state record")
	}
	if _, err := store.AcceptClosedProfile(first); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if _, err := store.CurrentRuntime(); err != nil {
		t.Fatalf("profile after exact retry: %v", err)
	}
}

func TestAcceptClosedProfileFailedConflictRetiresLiveOwner(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		name := "before state rename"
		if persisted {
			name = "after state rename"
		}
		t.Run(name, func(t *testing.T) {
			store, first := closedProfileStoreFixture(t)
			if _, err := store.AcceptClosedProfile(first); err != nil {
				t.Fatal(err)
			}
			if _, err := store.CurrentRuntime(); err != nil {
				t.Fatal(err)
			}
			authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
			generation := sha256.Sum256([]byte("closed profile generation"))
			candidate := store.current.Candidates[0]
			node := closedProfileNode{nodeID: candidate.NodeID, recordDigest: candidate.RecordDigest,
				domain: 2, subrole: 6, generation: candidate.RecordGeneration}
			second := testClosedProfile(t, authority, store.current.Snapshot.NetworkID, generation,
				store.current.Snapshot.Digest, store.config.clock(), []closedProfileNode{node})
			if sha256.Sum256(second) == sha256.Sum256(first) {
				t.Fatal("test did not produce a second digest")
			}
			failure := errors.New("injected conflict persistence failure")
			canceled := false
			store.workCancel = func() { canceled = true }
			_, err := store.acceptClosedProfileWithCommit(second, func(state durable.ClosedProfileState, raw []byte) error {
				if persisted {
					if err := store.storage.CommitClosedProfile(state, raw); err != nil {
						return err
					}
					return fmt.Errorf("%w: %w", durable.ErrClosedProfileStateSyncUncertain, failure)
				}
				return failure
			})
			if !errors.Is(err, failure) || !store.closed || !canceled {
				t.Fatalf("failed conflict: err=%v closed=%t canceled=%t", err, store.closed, canceled)
			}
			stored, raw, err := store.storage.LoadClosedProfile(generation)
			wantConflict := [32]byte{}
			if persisted {
				wantConflict = sha256.Sum256(second)
			}
			if err != nil || stored.Accepted != sha256.Sum256(first) || stored.Conflict != wantConflict || !bytes.Equal(raw, first) {
				t.Fatalf("durable conflict after failure: state=%+v bytes=%d err=%v", stored, len(raw), err)
			}
			if value, err := store.CurrentRuntime(); err == nil || value.Check(time.Now()) == nil {
				t.Fatalf("retired State exposed accepted profile: %+v, %v", value, err)
			}
			if value, err := store.CurrentRuntime(); err == nil || value.Check(time.Now()) == nil {
				t.Fatalf("retired State exposed route: %+v, %v", value, err)
			}
			if err := store.Wait(context.Background()); !errors.Is(err, failure) {
				t.Fatalf("Wait lost terminal cause: %v", err)
			}
			if err := store.Close(); !errors.Is(err, failure) {
				t.Fatalf("Close lost terminal cause: %v", err)
			}
		})
	}
}
