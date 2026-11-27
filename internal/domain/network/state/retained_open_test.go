package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func TestFreshStateAuthenticatesOnceAndRetainedOpeningKeepsHistory(t *testing.T) {
	genesis := newFixture(t)
	cfg := fixtureConfig(genesis, t.TempDir(), time.Unix(genesis.now, 0))
	owner, err := state.OpenFresh(cfg)
	if err != nil {
		t.Fatal(err)
	}
	acceptedReceipt, err := owner.AcceptEpoch(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations)
	if err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	accepted := acceptedReceipt.Snapshot
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if duplicate, err := state.OpenFresh(cfg); err == nil {
		_ = duplicate.Close()
		t.Fatal("fresh State adopted existing history")
	}
	owner, err = state.OpenRetained(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	current, err := owner.Current()
	if err != nil || current.Digest != accepted.Digest {
		t.Fatal("fresh acceptance lost at retained open", err)
	}
}

func TestRetainedStateWorkRefusalPrecedesRootIO(t *testing.T) {
	genesis := newFixture(t)
	root := filepath.Join(t.TempDir(), "absent-state")
	cfg := fixtureConfig(genesis, root, time.Unix(genesis.now, 0))
	refusal := errors.New("original application admission denied")
	cfg.PermitWork = func() error { return refusal }
	owner, err := state.OpenRetained(cfg)
	if owner != nil || !errors.Is(err, refusal) {
		if owner != nil {
			_ = owner.Close()
		}
		t.Fatal("retained opener bypassed original work refusal", err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("refused retained opener touched root", err)
	}
}

func TestRetainedStateReopensRealSignedHistoryAndKeepsSuccessorFloor(t *testing.T) {
	genesis := newFixture(t)
	cfg := fixtureConfig(genesis, t.TempDir(), time.Unix(genesis.now, 0))
	owner, err := state.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.AcceptEpoch(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatal(err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	for _, next := range []fixture{genesis, nextFixture(t, genesis)} {
		owner, err = state.OpenRetained(cfg)
		if err != nil {
			t.Fatal("retained signed owner refused", err)
		}
		if _, err = owner.AcceptEpoch(t.Context(), next.epoch, next.inputs, next.materializations); err != nil {
			t.Fatal(err)
		}
		view, err := owner.Current()
		if err != nil || view.Digest != next.epochDigest {
			t.Fatal("signed history changed at retained reopen", view, err)
		}
		if err = owner.Close(); err != nil {
			t.Fatal(err)
		}
	}
	owner, err = state.OpenRetained(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err = owner.AcceptEpoch(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err == nil {
		t.Fatal("retained reopen reset the successor floor")
	}
}

func TestRetainedStateDoesNotInitializeAbsentOrEmptiedRoot(t *testing.T) {
	genesis := newFixture(t)
	for _, empty := range []bool{false, true} {
		root := filepath.Join(t.TempDir(), "state")
		if empty {
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
		}
		cfg := fixtureConfig(genesis, root, time.Unix(genesis.now, 0))
		if owner, err := state.OpenRetained(cfg); err == nil {
			_ = owner.Close()
			t.Fatal("retained open initialized absent history")
		}
		if empty {
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("refused retained open recreated history", entries, err)
			}
		} else if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatal("refused retained open created root", err)
		}
	}
}

func TestRetainedServingStateDoesNotRecreateEmptiedParticipationHistory(t *testing.T) {
	// A real signed State and Source server establish and join their own first
	// participation history. This is a component consumer, not installed Startup.
	genesis := newFixture(t)
	cfg := installedServingSourceConfig(t, genesis)
	owner, err := state.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(cfg.LocalRoleStateRoot)
	if err != nil || len(entries) == 0 {
		t.Fatal("first Source did not establish real history", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			t.Fatal("unexpected test participation entry")
		}
		if err := os.Remove(filepath.Join(cfg.LocalRoleStateRoot, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	if successor, err := state.OpenRetained(cfg); err == nil {
		_ = successor.Close()
		t.Fatal("retained Source recreated empty participation history")
	}
	remaining, err := os.ReadDir(cfg.LocalRoleStateRoot)
	if err != nil || len(remaining) != 0 {
		t.Fatal("refused retained Source changed emptied history", remaining, err)
	}
}

func TestDistributionStateRepairsStaleCurrentMirror(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	clock := advancingVerificationClock(time.Unix(genesis.now, 0))
	config := state.Config{
		Root: t.TempDir(), NetworkID: genesis.networkID,
		Authorities:            map[[32]byte]ed25519.PublicKey{genesis.authorityID: genesis.authorityPublic},
		ClosedProfileAuthority: genesis.authorityPublic,
		Threshold:              1, Clock: clock, ObserveClock: clock,
	}
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	firstReceipt, err := store.AcceptEpoch(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations)
	if err != nil {
		t.Fatal(err)
	}
	first := firstReceipt.Snapshot
	secondReceipt, err := store.AcceptEpoch(context.Background(), successor.epoch, successor.inputs, successor.materializations)
	if err != nil {
		t.Fatal(err)
	}
	second := secondReceipt.Snapshot
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join(config.Root, "current")
	controlPath := filepath.Join(config.Root, "distribution", "current")
	control, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(config.Root, "distribution", "generations", strings.TrimSuffix(string(control), "\n"), "state.bin")
	journal, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte(first.Generation+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	restarted, err := state.OpenRetained(config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	current, err := restarted.Current()
	if err != nil {
		t.Fatalf("recover active generation: %v", err)
	}
	if current != second || current.Digest != successor.epochDigest {
		t.Fatalf("recovered active generation=%+v, want %+v", current, second)
	}
	if _, err := restarted.AcceptEpoch(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err == nil {
		t.Fatal("retained recovery accepted the signed predecessor")
	}
	for path, want := range map[string][]byte{
		currentPath: []byte(second.Generation + "\n"),
		controlPath: control,
		journalPath: journal,
		filepath.Join(config.Root, "generations", second.Generation, "epoch.bin"): successor.epoch,
	} {
		if raw, err := os.ReadFile(path); err != nil || !bytes.Equal(raw, want) {
			t.Errorf("retained bytes changed at %s: %v", path, err)
		}
	}
}
