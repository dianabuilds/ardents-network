package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestOfflineAcceptRejectsExpiredAtAcceptanceWithoutWriting(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	root := t.TempDir()
	now := time.Unix(fixtureNow, 0).UTC()
	store, err := state2.Open(state2.Config{
		Root: root, NetworkID: genesis.networkID,
		Authorities:            map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		ClosedProfileAuthority: genesis.authorityPublic,
		Threshold:              1, Clock: func() time.Time { return now }, ObserveClock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatalf("accept genesis: %v", err)
	}
	beforeCurrent, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	beforeDistribution, err := os.ReadFile(filepath.Join(root, "distribution", "current"))
	if err != nil {
		t.Fatal(err)
	}

	now = now.Add(2 * time.Hour)
	if _, err := store.Accept(context.Background(), successor.epoch, successor.inputs, successor.materializations); err == nil {
		t.Fatal("accepted successor expired at acceptance time")
	}
	afterCurrent, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	afterDistribution, err := os.ReadFile(filepath.Join(root, "distribution", "current"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterCurrent, beforeCurrent) || !bytes.Equal(afterDistribution, beforeDistribution) {
		t.Fatal("expired successor changed current pointer or distribution floor")
	}
}

func TestOfflineAcceptClockJumpLeavesRuntimeAndRefreshUsable(t *testing.T) {
	base := time.Unix(fixtureNow, 0).UTC()
	hour := base.Truncate(time.Hour)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{81}, 32))
	public := authority.Public().(ed25519.PublicKey)
	spec := networkfixture.ClosedSpec{
		NetworkID: [32]byte{1}, Seed: [32]byte{2}, IssuanceAuthority: [32]byte{3}, Number: 1,
		NotBefore: hour, NotAfter: hour.Add(2 * time.Hour), Authority: authority,
		Nodes: []networkfixture.ClosedNode{{RecordSpec: networkfixture.RecordSpec{
			NodeID: [32]byte{4}, Generation: 1, PrivateKey: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{82}, 32)),
			ValidFrom: hour, ValidUntil: hour.Add(2 * time.Hour), Endpoint: "127.0.0.1:4601",
			Carrier: "ardents-carrier-tcp-tls-v2", Capability: 2, Capacity: 8,
		}, RoleDomain: 2, Subrole: 6}},
	}
	signed, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	genesis := fixture{now: base.Unix(), networkID: spec.NetworkID, authorityID: sha256.Sum256(public),
		authorityPublic: public, epoch: signed.Epoch.Raw, epochDigest: signed.Epoch.Digest,
		inputs: signed.Epoch.Inputs, materializations: signed.Epoch.Materials[:1]}
	cfg, stopSources := sourceEnvironment(t, genesis, genesis, genesis)
	defer stopSources()
	elapsedClock := advancingVerificationClock(base)
	var wallOffset time.Duration
	cfg.Clock = func() time.Time { return elapsedClock().Add(wallOffset) }
	cfg.ObserveClock = elapsedClock
	owner, err := state2.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	if _, err := owner.AcceptClosedProfile(signed.Profile); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.CurrentRuntime(); err != nil {
		t.Fatalf("runtime positive control: %v", err)
	}
	control := filepath.Join(cfg.Root, "distribution", "current")
	before, err := os.ReadFile(control)
	if err != nil {
		t.Fatal(err)
	}
	wallOffset = 10 * time.Minute
	if _, err := owner.Accept(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err == nil || !strings.Contains(err.Error(), "clock confidence") {
		t.Fatalf("offline replay admitted a wall-only jump: %v", err)
	}
	wallOffset = 0
	after, err := os.ReadFile(control)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("clock refusal changed the durable journal: %v", err)
	}
	if _, err := owner.CurrentRuntime(); err != nil {
		t.Fatalf("runtime did not recover when wall time was restored: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := state2.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if view, err := reopened.CurrentRuntime(); err != nil || view.Profile().EpochDigest != genesis.epochDigest {
		t.Fatalf("reopened Runtime: %v", err)
	}
	if snap, err := reopened.Refresh(t.Context()); err != nil || snap.Digest != genesis.epochDigest || snap.SourceAttempts != 2 {
		t.Fatalf("reopened Refresh: %+v, %v", snap, err)
	}
}

func TestOfflineAcceptRequiresBothClockBoundsAtValidFrom(t *testing.T) {
	original := newFixture(t)
	at := time.Unix(fixtureNow+10, 0).UTC()
	future := buildFixtureEpoch(t, original, 1, [32]byte{}, sha256.Sum256([]byte("future genesis")),
		at, at.Add(time.Minute))
	for _, trial := range []struct {
		name                      string
		anchor, wall, independent time.Time
		wantCurrent               bool
	}{
		{"future genesis", at.Add(-10 * time.Second), at.Add(-10 * time.Second), at.Add(-10 * time.Second), false},
		{"independent below boundary", at, at, at.Add(-time.Second), false},
		{"monotonic below boundary", at.Add(-time.Second), at, at, false},
		{"at confirmed boundary", at, at, at, true},
	} {
		t.Run(trial.name, func(t *testing.T) {
			wall := trial.anchor
			cfg := fixtureConfig(original, t.TempDir(), time.Time{})
			cfg.Clock = func() time.Time { return wall }
			cfg.ClockObservation = trial.independent
			owner, err := state2.Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer owner.Close()
			wall = trial.wall
			snapshot, err := owner.Accept(t.Context(), future.epoch, future.inputs, future.materializations)
			if trial.wantCurrent {
				if err != nil || snapshot.Digest != future.epochDigest || !snapshot.TrustedTime.IsZero() {
					t.Fatalf("confirmed boundary: %+v, %v", snapshot, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "not yet current") {
				t.Fatalf("unreached boundary: %v", err)
			}
			if _, err := owner.Current(); !errors.Is(err, state2.ErrNoCurrentGeneration) {
				t.Fatalf("future genesis acquired current state: %v", err)
			}
			for _, name := range []string{"current", "distribution/current"} {
				if _, err := os.Stat(filepath.Join(cfg.Root, name)); !os.IsNotExist(err) {
					t.Fatalf("future genesis wrote %s: %v", name, err)
				}
			}
		})
	}
}
