package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// F-50/ADR-0111 evidence: AREP v3 is the sole new-candidate closed intake
// schema; retained old-schema current/pending roots refuse with the typed
// recovery outcome and keep their floors; predecessor chain members stay
// authenticated by the historical verifier. The vectors come from this
// package's own canonical builders, as its package-map row requires.

type closedIntakeEpoch struct {
	testEpoch
	inputs [][]byte
}

func closedIntakeKeys() ([32]byte, ed25519.PrivateKey) {
	network := sha256.Sum256([]byte("closed intake network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x5a}, ed25519.SeedSize))
	return network, authority
}

func buildClosedIntakeEpoch(t *testing.T, network [32]byte, authority ed25519.PrivateKey,
	number uint64, previous [32]byte, version byte, from, until time.Time) closedIntakeEpoch {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(0x30 + number)}, ed25519.SeedSize))
	record := buildTestRecordWithCarrier(t, network, sha256.Sum256([]byte{byte(number)}), key,
		"intake-family", "127.0.0.1:4700", "ardents-carrier-tcp-tls-v2", 4, 2, from, until)
	inputs := [][]byte{record.bytes}
	built := buildTestEpoch(t, testEpochSpec{networkID: network, number: number, previous: previous,
		validFrom: from, validUntil: until, inputs: inputs, accepted: []fixtureRecord{record},
		assignmentSeed: sha256.Sum256([]byte("closed intake seed")), domains: []string{"intake"},
		authorities: []ed25519.PrivateKey{authority}, profile: "ardents-route-v3", version: version})
	return closedIntakeEpoch{testEpoch: built, inputs: inputs}
}

func closedIntakeConfig(root string, network [32]byte, authority ed25519.PrivateKey, now time.Time) state2.Config {
	public := authority.Public().(ed25519.PublicKey)
	return state2.Config{Root: root, NetworkID: network,
		Authorities:            map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
		Threshold:              1,
		ClosedProfileAuthority: public,
		AcceptedProfile:        "ardents-route-v3",
		Now:                    now, ClockObservation: now}
}

func TestClosedOfflineCurrentConflictPersistsOnlyAuthenticatedEvidence(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	genesis := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 3, now.Add(-time.Minute), now.Add(time.Hour))
	alternative := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 3, now.Add(-2*time.Minute), now.Add(time.Hour))
	successor := buildClosedIntakeEpoch(t, network, authority, 2, genesis.Digest, 3, now.Add(-time.Minute), now.Add(time.Hour))
	wrongNetwork := buildClosedIntakeEpoch(t, sha256.Sum256([]byte("unrelated closed Network")), authority, 1, [32]byte{}, 3, now.Add(-time.Minute), now.Add(time.Hour))
	badSignature := alternative
	badSignature.Raw = bytes.Clone(alternative.Raw)
	badSignature.Raw[len(badSignature.Raw)-1] ^= 1
	for _, test := range []struct {
		name     string
		input    closedIntakeEpoch
		conflict bool
	}{{"signed-conflict", alternative, true}, {"bad-signature", badSignature, false}, {"wrong-network", wrongNetwork, false}} {
		t.Run(test.name, func(t *testing.T) {
			config := closedIntakeConfig(t.TempDir(), network, authority, now)
			owner, err := state2.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			if _, err := owner.Accept(context.Background(), genesis.Raw, genesis.inputs, genesis.Materials); err != nil {
				t.Fatal(err)
			}
			if _, err := owner.Accept(context.Background(), test.input.Raw, test.input.inputs, test.input.Materials); err == nil {
				t.Fatal("invalid or conflicting evidence was accepted")
			}
			view, err := owner.Current()
			if err != nil || view.Conflicting != test.conflict || view.Digest != genesis.Digest {
				t.Fatalf("retained identity/conflict: epoch=%d conflict=%t err=%v", view.Epoch, view.Conflicting, err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			owner, err = state2.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			view, err = owner.Current()
			if err != nil || view.Conflicting != test.conflict || view.Digest != genesis.Digest {
				t.Fatalf("reopen lost identity/conflict: epoch=%d conflict=%t err=%v", view.Epoch, view.Conflicting, err)
			}
			_, err = owner.Accept(context.Background(), successor.Raw, successor.inputs, successor.Materials)
			if test.conflict && err == nil {
				t.Fatal("successor erased retained conflict")
			}
			if !test.conflict && err != nil {
				t.Fatalf("unauthenticated input poisoned successor acceptance: %v", err)
			}
		})
	}
}

func TestClosedIntakeRefusesRetiredSchemasBeforeCommit(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	for _, version := range []byte{1, 2} {
		root := t.TempDir()
		store, err := state2.Open(closedIntakeConfig(root, network, authority, now))
		if err != nil {
			t.Fatal(err)
		}
		epoch := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, version, now.Add(-time.Minute), now.Add(time.Hour))
		_, err = store.Accept(context.Background(), epoch.Raw, epoch.inputs, epoch.Materials)
		if !errors.Is(err, state2.ErrLegacyEpochIntake) {
			t.Fatalf("v%d closed Accept = %v", version, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		reopened, err := state2.Open(closedIntakeConfig(root, network, authority, now))
		if err != nil {
			t.Fatalf("reopen after v%d refusal: %v", version, err)
		}
		if _, err := reopened.Current(); !errors.Is(err, state2.ErrNoCurrentGeneration) {
			t.Fatalf("refused v%d left a committed generation: %v", version, err)
		}
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// The same harness accepts the sole pinned schema.
	root := t.TempDir()
	store, err := state2.Open(closedIntakeConfig(root, network, authority, now))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	epoch := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 3, now.Add(-time.Minute), now.Add(time.Hour))
	snapshot, err := store.Accept(context.Background(), epoch.Raw, epoch.inputs, epoch.Materials)
	if err != nil || snapshot.Epoch != 1 || snapshot.Generation != hex.EncodeToString(epoch.Digest[:]) {
		t.Fatalf("v3 closed Accept = %+v, %v", snapshot, err)
	}
}

func TestClosedIntakeClassifiesRetiredCurrentOnReopen(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	for _, version := range []byte{1, 2} {
		root := t.TempDir()
		epoch := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, version, now.Add(-time.Minute), now.Add(time.Hour))
		name := hex.EncodeToString(epoch.Digest[:])
		// A historical build committed this root before the intake was pinned.
		if err := state2.CommitRetainedGenerationForTest(root, name, epoch.Raw, epoch.inputs, true); err != nil {
			t.Fatal(err)
		}
		_, err := state2.Open(closedIntakeConfig(root, network, authority, now))
		var required *state2.RecoveryRequiredError
		if !errors.As(err, &required) || !strings.Contains(required.Reason, "retired AREP schema") {
			t.Fatalf("reopen with retained v%d current = %v", version, err)
		}
		for _, floor := range []string{filepath.Join("generations", name), "current"} {
			if _, statErr := os.Lstat(filepath.Join(root, floor)); statErr != nil {
				t.Fatalf("retired current floor %s lost: %v", floor, statErr)
			}
		}
	}
}

func TestClosedIntakeClassifiesRetiredPendingOnReopen(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	root := t.TempDir()
	current := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 3, now.Add(-time.Minute), now.Add(time.Hour))
	pending := buildClosedIntakeEpoch(t, network, authority, 2, current.Digest, 1, now.Add(-time.Minute), now.Add(2*time.Hour))
	if err := state2.CommitRetainedGenerationForTest(root, hex.EncodeToString(current.Digest[:]), current.Raw, current.inputs, true); err != nil {
		t.Fatal(err)
	}
	pendingName := hex.EncodeToString(pending.Digest[:])
	if err := state2.CommitRetainedGenerationForTest(root, pendingName, pending.Raw, pending.inputs, false); err != nil {
		t.Fatal(err)
	}
	pendingFrom := now.Add(-time.Minute).Unix()
	if err := state2.CommitRetainedControlForTest(root, 1, current.Digest, pending.Digest, pendingFrom); err != nil {
		t.Fatal(err)
	}
	_, err := state2.Open(closedIntakeConfig(root, network, authority, now))
	var required *state2.RecoveryRequiredError
	if !errors.As(err, &required) || !strings.Contains(required.Reason, "pending") {
		t.Fatalf("reopen with retired pending = %v", err)
	}
	for _, floor := range []string{filepath.Join("generations", pendingName), "current", filepath.Join("distribution", "current")} {
		if _, statErr := os.Lstat(filepath.Join(root, floor)); statErr != nil {
			t.Fatalf("pending floor %s lost: %v", floor, statErr)
		}
	}
}

func TestClosedIntakeKeepsAuthenticatedOldPredecessorChain(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	root := t.TempDir()
	from, until := now.Add(-time.Minute), now.Add(time.Hour)
	first := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 1, from, until)
	second := buildClosedIntakeEpoch(t, network, authority, 2, first.Digest, 2, from, until)
	third := buildClosedIntakeEpoch(t, network, authority, 3, second.Digest, 3, from, until)
	retained := []closedIntakeEpoch{first, second, third}
	for index, epoch := range retained {
		if err := state2.CommitRetainedGenerationForTest(root, hex.EncodeToString(epoch.Digest[:]),
			epoch.Raw, epoch.inputs, index == len(retained)-1); err != nil {
			t.Fatal(err)
		}
	}
	if err := state2.CommitRetainedControlForTest(root, 3, third.Digest, [32]byte{}, 0); err != nil {
		t.Fatal(err)
	}
	store, err := state2.Open(closedIntakeConfig(root, network, authority, now))
	if err != nil {
		t.Fatalf("v3 current with v1/v2 predecessors refused: %v", err)
	}
	defer store.Close()
	snapshot, err := store.Current()
	if err != nil || snapshot.Epoch != 3 || snapshot.Profile != "ardents-route-v3" {
		t.Fatalf("retained chain current = %+v, %v", snapshot, err)
	}
	// The retained chain still extends through the sole pinned schema.
	fourth := buildClosedIntakeEpoch(t, network, authority, 4, third.Digest, 3, from, until)
	accepted, err := store.Accept(context.Background(), fourth.Raw, fourth.inputs, fourth.Materials)
	if err != nil || accepted.Epoch != 4 {
		t.Fatalf("successor Accept over old predecessors = %+v, %v", accepted, err)
	}
}

func TestClosedIntakeSourceChokePointCoversBothResultForms(t *testing.T) {
	t.Parallel()
	network, authority := closedIntakeKeys()
	now := time.Unix(1_800_000_100, 0).UTC()
	root := t.TempDir()
	store, err := state2.Open(closedIntakeConfig(root, network, authority, now))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	from, until := now.Add(-time.Minute), now.Add(time.Hour)
	current := buildClosedIntakeEpoch(t, network, authority, 1, [32]byte{}, 3, from, until)
	if _, err := store.Accept(context.Background(), current.Raw, current.inputs, current.Materials); err != nil {
		t.Fatal(err)
	}
	// Reuse form: the exact retained current bytes stay a valid wave result.
	if err := state2.VerifySourceCandidateForTest(store, current.Raw, current.inputs, current.Materials); err != nil {
		t.Fatalf("exact current reuse refused: %v", err)
	}
	// New-candidate form: retired schemas never become valid wave results.
	for _, version := range []byte{1, 2} {
		legacy := buildClosedIntakeEpoch(t, network, authority, 2, current.Digest, version, from, until)
		if err := state2.VerifySourceCandidateForTest(store, legacy.Raw, legacy.inputs, legacy.Materials); !errors.Is(err, state2.ErrLegacyEpochIntake) {
			t.Fatalf("source v%d candidate = %v", version, err)
		}
	}
	// The pinned schema still passes the same choke point.
	next := buildClosedIntakeEpoch(t, network, authority, 2, current.Digest, 3, from, until)
	if err := state2.VerifySourceCandidateForTest(store, next.Raw, next.inputs, next.Materials); err != nil {
		t.Fatalf("source v3 candidate refused: %v", err)
	}
}
