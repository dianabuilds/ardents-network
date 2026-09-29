package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestUncertainControlCommitRetiresLiveStateAndRetainsFloor(t *testing.T) {
	s, current := newControlCommitFixture(t, false)
	canceled := false
	s.workCancel = func() { canceled = true }
	before := s.distribution
	next := before
	next.sequence++
	next.trustedTimeFloor++
	failure := fmt.Errorf("%w: injected after control pointer replacement", durable.ErrPointerSyncUncertain)
	err := s.commitDistributionWithControl(next, func(name string, raw []byte) error {
		if err := s.storage.CommitControl(name, raw); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, durable.ErrPointerSyncUncertain) || !s.closed || !canceled || s.distribution.sequence != before.sequence {
		t.Fatalf("uncertain control commit: err=%v closed=%t canceled=%t in-memory sequence=%d", err, s.closed, canceled, s.distribution.sequence)
	}
	if _, err := s.Current(); err == nil {
		t.Fatal("Current served a predecessor after uncertain control commit")
	}
	request := source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(s.config.networkID)}
	if response := s.resolveDistributionRequest(context.Background(), request); response.Status != "busy" || len(response.Payload) != 0 {
		t.Fatalf("Source response after uncertain control commit: %+v", response)
	}
	if err := s.commitDistribution(next); err == nil {
		t.Fatal("retired State wrote another Source-cycle step")
	}
	if err := s.Wait(context.Background()); !errors.Is(err, durable.ErrPointerSyncUncertain) {
		t.Fatalf("Wait lost terminal control failure: %v", err)
	}
	_, raw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	stored, err := decodeDistributionState(raw)
	if err != nil || stored.sequence != next.sequence || stored.trustedTimeFloor != next.trustedTimeFloor {
		t.Fatalf("visible durable floor: sequence=%d trusted=%d err=%v", stored.sequence, stored.trustedTimeFloor, err)
	}
	if err := s.Close(); !errors.Is(err, durable.ErrPointerSyncUncertain) {
		t.Fatalf("Close lost terminal control failure: %v", err)
	}
	storage, err := openTestDurableRoot(s.config.root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	reopened := &networkState{config: s.config, storage: storage, current: &current}
	if err := reopened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	if reopened.distribution.sequence != next.sequence || reopened.distribution.trustedTimeFloor != next.trustedTimeFloor {
		t.Fatal("reopen did not retain the visible control floor")
	}
}

func TestFailedNewConflictControlCommitRetiresPriorDecision(t *testing.T) {
	s, current := newControlCommitFixture(t, true)
	before := s.distribution
	beforeName, beforeRaw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	canceled := false
	s.workCancel = func() { canceled = true }
	next := before
	next.sequence++
	next.conflicting = true
	failure := errors.New("injected pre-pointer conflict commit failure")
	err = s.commitDistributionWithControl(next, func(string, []byte) error { return failure })
	if !errors.Is(err, failure) || !s.closed || !canceled ||
		s.distribution.sequence != before.sequence || s.distribution.conflicting != before.conflicting {
		t.Fatalf("failed conflict commit: err=%v closed=%t canceled=%t state=%+v", err, s.closed, canceled, s.distribution)
	}
	if _, err := s.Current(); err == nil {
		t.Fatal("Current served the predecessor after a verified conflict could not be persisted")
	}
	for _, operation := range []string{"latest", "by-digest"} {
		request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(s.config.networkID), ObjectDigest: current.Header.Digest}
		response := s.resolveDistributionRequest(context.Background(), request)
		if response.Status != "busy" || response.ObjectDigest != [32]byte{} || len(response.Payload) != 0 {
			t.Fatalf("%s Source response after failed conflict: %+v", operation, response)
		}
	}
	if _, err := s.Accept(context.Background(), nil, nil, nil); err == nil {
		t.Fatal("retired owner admitted another offline candidate")
	}
	if _, err := s.Refresh(context.Background()); err == nil {
		t.Fatal("retired owner started another manual Source wave")
	}
	if err := s.Wait(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Wait lost the failed conflict cause: %v", err)
	}
	name, raw, err := s.storage.LoadControl()
	if err != nil || name != beforeName || !bytes.Equal(raw, beforeRaw) {
		t.Fatalf("failed conflict changed the durable control floor: name=%q err=%v", name, err)
	}
	currentName, _, err := s.storage.LoadState()
	if err != nil || currentName != current.Snapshot.Generation {
		t.Fatalf("failed conflict changed the authenticated current generation: %q, %v", currentName, err)
	}
	family := sha256.Sum256([]byte(current.Snapshot.DeclaredFamily))
	protected, err := duty.ReadConflict(s.config.localRoles, s.config.clock, current.Snapshot.NodeID, family)
	if err != nil || !protected {
		t.Fatalf("retired Source lost its guard before Close: protected=%t err=%v", protected, err)
	}
	if err := s.Close(); !errors.Is(err, failure) {
		t.Fatalf("Close lost the failed conflict cause: %v", err)
	}
}

func TestFailedOrdinaryControlCommitRemainsRetryable(t *testing.T) {
	s, current := newControlCommitFixture(t, false)
	before := s.distribution
	next := before
	next.sequence++
	failure := errors.New("injected ordinary pre-pointer failure")
	if err := s.commitDistributionWithControl(next, func(string, []byte) error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("ordinary control failure = %v", err)
	}
	if s.closed || s.distribution.sequence != before.sequence {
		t.Fatalf("ordinary control failure retired owner: closed=%t state=%+v", s.closed, s.distribution)
	}
	if snapshot, err := s.Current(); err != nil || snapshot.Digest != current.Header.Digest {
		t.Fatalf("ordinary control failure lost prior current: %+v, %v", snapshot, err)
	}
	if err := s.commitDistribution(next); err != nil || s.distribution.sequence != next.sequence {
		t.Fatalf("ordinary control retry: state=%+v err=%v", s.distribution, err)
	}
}

func TestOfflineVerifiedPendingConflictCommitFailureRetiresOwner(t *testing.T) {
	s, competing, _ := newVerifiedPendingConflictFixture(t)
	failure := errors.New("injected offline conflict pre-pointer failure")
	before := s.distribution
	name, raw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.acceptWithConflictCommit(context.Background(), competing.Raw, competing.Inputs, competing.Materials[:1],
		func(string, []byte) error { return failure })
	if !errors.Is(err, failure) || !s.closed || s.distribution.sequence != before.sequence {
		t.Fatalf("verified offline conflict failure: err=%v closed=%t state=%+v", err, s.closed, s.distribution)
	}
	assertFailedVerifiedConflict(t, s, failure, name, raw)
}

func TestManualSourceVerifiedPendingConflictCommitFailureRetiresOwner(t *testing.T) {
	s, _, competing := newVerifiedPendingConflictFixture(t)
	failure := errors.New("injected Source conflict pre-pointer failure")
	wave := s.distribution
	wave.sequence++
	wave.cycleID++
	wave.cycleActive = true
	wave.cyclePurpose = sourceCyclePurposeRefresh
	wave.cycleStarted = s.config.now.Unix()
	wave.cycleDeadline = s.config.now.Add(sourceWaveDuration).Unix()
	wave.cycleSeed = sha256.Sum256([]byte("manual Source conflict cycle"))
	wave.sourceOrder = [2]byte{0, 1}
	wave.attempts[0], wave.attempts[1] = sourceAttemptInFlight, sourceAttemptInFlight
	if err := s.commitDistribution(wave); err != nil {
		t.Fatal(err)
	}
	before := s.distribution
	name, raw, err := s.storage.LoadControl()
	if err != nil {
		t.Fatal(err)
	}
	s.refreshing = true
	_, err = s.completeSourceWaveWithConflictCommit(s.config.now, s.current, []sourceResult{
		{slot: 0, decision: competing, observations: [4]byte{sourceOutcomeValid}},
		{slot: 1, decision: competing, observations: [4]byte{0, sourceOutcomeValid}},
	}, func(string, []byte) error { return failure })
	if !errors.Is(err, failure) || !s.closed || s.refreshing || s.distribution.sequence != before.sequence {
		t.Fatalf("verified Source conflict failure: err=%v closed=%t refreshing=%t state=%+v", err, s.closed, s.refreshing, s.distribution)
	}
	assertFailedVerifiedConflict(t, s, failure, name, raw)
}

func assertFailedVerifiedConflict(t *testing.T, s *networkState, failure error, name string, raw []byte) {
	t.Helper()
	if _, err := s.Current(); err == nil {
		t.Fatal("retired State served the predecessor")
	}
	for _, operation := range []string{"latest", "by-digest"} {
		request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(s.config.networkID), ObjectDigest: s.current.Header.Digest}
		response := s.resolveDistributionRequest(context.Background(), request)
		if response.Status != "busy" || response.ObjectDigest != [32]byte{} || len(response.Payload) != 0 {
			t.Fatalf("retired %s Source response: %+v", operation, response)
		}
	}
	if err := s.Wait(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Wait lost failed conflict cause: %v", err)
	}
	retainedName, retainedRaw, err := s.storage.LoadControl()
	if err != nil || retainedName != name || !bytes.Equal(retainedRaw, raw) {
		t.Fatalf("failed conflict changed durable control: name=%q err=%v", retainedName, err)
	}
	if err := s.Close(); !errors.Is(err, failure) {
		t.Fatalf("Close lost failed conflict cause: %v", err)
	}
}

func newVerifiedPendingConflictFixture(t *testing.T) (*networkState, networkfixture.Epoch, epoch.Decision) {
	t.Helper()
	now := time.Unix(1_800_000_100, 0).UTC()
	networkID := sha256.Sum256([]byte("verified pending conflict commit network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x69}, ed25519.SeedSize))
	public := authority.Public().(ed25519.PublicKey)
	recordKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x47}, ed25519.SeedSize))
	record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
		NetworkID: networkID, NodeID: [32]byte{0x47}, Generation: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour),
		Family: "conflict-test-family", Endpoint: "127.0.0.1:4101", Capability: 1, Capacity: 3, PrivateKey: recordKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	build := func(number uint64, previous [32]byte, from time.Time, seed string) networkfixture.Epoch {
		t.Helper()
		built, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
			NetworkID: networkID, Number: number, Previous: previous,
			ValidFrom: from, ValidUntil: now.Add(30 * time.Minute),
			Inputs: [][]byte{record.Raw}, Accepted: []networkfixture.Record{record},
			AssignmentSeed: sha256.Sum256([]byte(seed)), Domains: []string{"alpha"},
			Authorities: []ed25519.PrivateKey{authority},
		})
		if err != nil {
			t.Fatal(err)
		}
		return built
	}
	genesis := build(1, [32]byte{}, now.Add(-time.Minute), "genesis")
	s, err := Open(Config{Root: t.TempDir(), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
		Threshold:   1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.Accept(context.Background(), genesis.Raw, genesis.Inputs, genesis.Materials[:1]); err != nil {
		t.Fatalf("accept signed genesis: %v", err)
	}
	pending := build(2, genesis.Digest, now.Add(20*time.Second), "pending")
	competing := build(2, genesis.Digest, now.Add(-time.Minute), "competing")
	verify := func(candidate networkfixture.Epoch, at time.Time) epoch.Decision {
		t.Helper()
		verification := s.config
		verification.now = at
		decision, err := verifyDecision(verification, epochPredecessor(s.current), candidate.Raw, candidate.Inputs, candidate.Materials[:1], true)
		if err != nil {
			t.Fatalf("verify signed successor: %v", err)
		}
		return decision
	}
	pendingDecision := verify(pending, now.Add(20*time.Second))
	competingDecision := verify(competing, now)
	if err := stageGeneration(s.storage, pendingDecision); err != nil {
		t.Fatal(err)
	}
	state := s.distribution
	state.sequence++
	state.pendingDigest, state.pendingValidFrom = pending.Digest, pendingDecision.Header.ValidFrom.Unix()
	if err := s.commitDistribution(state); err != nil {
		t.Fatal(err)
	}
	s.pendingDecision = &pendingDecision
	return s, competing, competingDecision
}

func TestPersistedConflictKeepsDiagnosticSnapshotButRefusesSource(t *testing.T) {
	s, current := newControlCommitFixture(t, true)
	requests := []source.Message{
		{Operation: "latest", NetworkDigest: source.NetworkDigest(s.config.networkID)},
		{Operation: "by-digest", NetworkDigest: source.NetworkDigest(s.config.networkID), ObjectDigest: current.Header.Digest},
	}
	for _, request := range requests {
		response := s.resolveDistributionRequest(context.Background(), request)
		if response.Status != "ok" || response.ObjectDigest != current.Header.Digest || len(response.Payload) == 0 {
			t.Fatalf("healthy %s Source response: %+v", request.Operation, response)
		}
	}
	next := s.distribution
	next.sequence++
	next.conflicting = true
	if err := s.commitDistribution(next); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := s.Current(); err != nil || !snapshot.Conflicting || snapshot.Freshness != "conflicting" {
		t.Fatalf("persisted conflict diagnostic: %+v, %v", snapshot, err)
	}
	assertBusy := func(owner *networkState) {
		t.Helper()
		for _, request := range requests {
			response := owner.resolveDistributionRequest(context.Background(), request)
			if response.Status != "busy" || response.ObjectDigest != [32]byte{} || len(response.Payload) != 0 {
				t.Fatalf("conflicting %s Source response: %+v", request.Operation, response)
			}
		}
	}
	assertBusy(s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	storage, err := openTestDurableRoot(s.config.root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	reopened := &networkState{config: s.config, storage: storage, current: &current}
	if err := reopened.loadDistributionState(); err != nil {
		t.Fatal(err)
	}
	if snapshot, err := reopened.Current(); err != nil || !snapshot.Conflicting || snapshot.Freshness != "conflicting" {
		t.Fatalf("recovered conflict diagnostic: %+v, %v", snapshot, err)
	}
	assertBusy(reopened)
}

func TestActiveControlCommitFailureKeepsCorrectSourceDuty(t *testing.T) {
	for _, test := range []struct {
		name      string
		uncertain bool
		active    bool
	}{
		{name: "before pointer replacement"},
		{name: "before pointer replacement with handler", active: true},
		{name: "after pointer replacement", uncertain: true},
		{name: "after pointer replacement with handler", uncertain: true, active: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, current := newControlCommitFixture(t, true)
			if test.active {
				s.activeSource = 1
			}
			successor := current
			successor.Header.Number++
			successor.Header.Digest = sha256.Sum256([]byte("control-commit-successor"))
			successor.Header.ValidUntil = current.Header.ValidUntil.Add(time.Hour)
			successor.Snapshot.Epoch++
			successor.Snapshot.Digest = successor.Header.Digest
			successor.Snapshot.Generation = hex.EncodeToString(successor.Header.Digest[:])
			successor.Snapshot.NodeID = sha256.Sum256([]byte("new-serving-source"))
			successor.Snapshot.DeclaredFamily = "new-serving-family"
			successor.Snapshot.ValidUntil = current.Snapshot.ValidUntil.Add(time.Hour)
			next := s.distribution
			next.sequence++
			failure := errors.New("injected before control pointer replacement")
			commit := func(name string, raw []byte) error { return failure }
			if test.uncertain {
				failure = fmt.Errorf("%w: injected after control pointer replacement", durable.ErrPointerSyncUncertain)
				commit = func(name string, raw []byte) error {
					if err := s.storage.CommitControl(name, raw); err != nil {
						return err
					}
					return failure
				}
			}
			if err := s.commitActiveDecisionWithControl(successor, next, commit); !errors.Is(err, failure) {
				t.Fatalf("active commit failure = %v, want %v", err, failure)
			}
			clock := s.config.clock
			oldFamily := sha256.Sum256([]byte(current.Snapshot.DeclaredFamily))
			newFamily := sha256.Sum256([]byte(successor.Snapshot.DeclaredFamily))
			oldProtected, err := duty.ReadConflict(s.config.localRoles, clock, current.Snapshot.NodeID, oldFamily)
			if err != nil {
				t.Fatal(err)
			}
			newProtected, err := duty.ReadConflict(s.config.localRoles, clock, successor.Snapshot.NodeID, newFamily)
			if err != nil {
				t.Fatal(err)
			}
			if test.uncertain {
				if !s.closed || oldProtected != test.active || !newProtected {
					t.Fatalf("uncertain Source guard: closed=%t old=%t successor=%t", s.closed, oldProtected, newProtected)
				}
				if _, err := s.Current(); err == nil {
					t.Fatal("uncertain owner served predecessor State")
				}
				request := source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(s.config.networkID)}
				if response := s.resolveDistributionRequest(context.Background(), request); response.Status != "busy" {
					t.Fatalf("uncertain owner served Source: %q", response.Status)
				}
				roles, err := duty.Open(duty.Config{Root: s.config.localRoles, Clock: clock})
				if err != nil {
					t.Fatal(err)
				}
				collision := duty.Duty{Identity: successor.Snapshot.NodeID, Family: newFamily,
					Class: "node-duty", State: "live", NotAfter: successor.Snapshot.ValidUntil}
				replaceErr := roles.Replace([32]byte{88}, []duty.Duty{collision})
				closeErr := roles.Close()
				if !errors.Is(replaceErr, duty.ErrLocalRoleConflict) || closeErr != nil {
					t.Fatalf("successor collision guard: replace=%v close=%v", replaceErr, closeErr)
				}
				if err := s.Close(); !errors.Is(err, durable.ErrPointerSyncUncertain) {
					t.Fatalf("Close lost uncertain commit cause: %v", err)
				}
			} else {
				if s.closed || !oldProtected || newProtected {
					t.Fatalf("pre-pointer Source guard: closed=%t old=%t successor=%t", s.closed, oldProtected, newProtected)
				}
				if snapshot, err := s.Current(); err != nil || snapshot.Digest != current.Header.Digest {
					t.Fatalf("pre-pointer Current changed: %+v, %v", snapshot, err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func newControlCommitFixture(t *testing.T, serving bool) (*networkState, epoch.Decision) {
	t.Helper()
	decision, networkID := sourceResolverGoldenDecision(t)
	root := t.TempDir()
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_100, 0).UTC()
	s := &networkState{storage: storage, current: &decision,
		config: config{root: root, networkID: networkID, clock: func() time.Time { return now }, observe: func() time.Time { return now }, anchorWall: now, anchorMono: time.Now(),
			localRoles: filepath.Join(t.TempDir(), "roles"), sourceInfo: source.Details{Serving: serving}}}
	if err := persistDecision(storage, decision, true); err != nil {
		_ = storage.Close()
		t.Fatal(err)
	}
	initial := distributionState{sequence: 1, epochFloor: decision.Header.Number, epochDigest: decision.Header.Digest}
	if err := s.commitDistribution(initial); err != nil {
		_ = storage.Close()
		t.Fatal(err)
	}
	if serving {
		if err := s.retainSourceServer(); err != nil {
			_ = storage.Close()
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, decision
}
