package state

import (
	"context"
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

func TestActiveControlCommitFailureKeepsCorrectSourceDuty(t *testing.T) {
	for _, test := range []struct {
		name      string
		uncertain bool
	}{
		{name: "before pointer replacement"},
		{name: "after pointer replacement", uncertain: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, current := newControlCommitFixture(t, true)
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
				if !s.closed || oldProtected || !newProtected {
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
