package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestRetainedStateReopensRealSignedHistoryAndKeepsSuccessorFloor(t *testing.T) {
	genesis := newFixture(t)
	cfg := fixtureConfig(genesis, t.TempDir(), time.Unix(genesis.now, 0))
	owner, err := state.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Accept(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
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
		if _, err = owner.Accept(t.Context(), next.epoch, next.inputs, next.materializations); err != nil {
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
	if _, err = owner.Accept(t.Context(), genesis.epoch, genesis.inputs, genesis.materializations); err == nil {
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
