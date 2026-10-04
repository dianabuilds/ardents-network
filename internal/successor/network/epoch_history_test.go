package network_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestEpochHistoryRetainsPendingConflictBeforeWinnerSelection(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	current := network.EpochFacts{Network: [32]byte{1}, Digest: [32]byte{2}, Number: 1, ValidFrom: now, ValidUntil: now.Add(time.Hour)}
	pending := network.EpochFacts{Network: current.Network, Digest: [32]byte{3}, Previous: current.Digest,
		Number: 2, ValidFrom: now.Add(time.Minute), ValidUntil: now.Add(2 * time.Hour)}
	history, err := network.RestoreEpochHistory(current, pending, false)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := history.Reconcile([]network.EpochFacts{current, pending})
	if err != nil || selection.Index() != 1 {
		t.Fatalf("successor selection: %v", err)
	}
	if future, err := selection.DispositionAt(now); err != nil || future != network.EpochPending {
		t.Fatalf("premature activation: %v", err)
	}
	if future, err := selection.DispositionAt(pending.ValidFrom); err != nil || future != network.EpochCurrent {
		t.Fatalf("pending did not activate: %v", err)
	}
	if _, err := selection.DispositionAt(pending.ValidUntil); !errors.Is(err, network.ErrEpochExpired) {
		t.Fatalf("expired successor: %v", err)
	}
	conflict := current
	conflict.Digest = [32]byte{4}
	for _, candidates := range [][]network.EpochFacts{{conflict, pending}, {pending, conflict}} {
		if _, err := history.Reconcile(candidates); !errors.Is(err, network.ErrEpochConflict) {
			t.Fatalf("newest hid conflict: %v", err)
		}
	}
	conflict = pending
	conflict.Digest = [32]byte{5}
	if _, err := history.Reconcile([]network.EpochFacts{conflict}); !errors.Is(err, network.ErrEpochConflict) {
		t.Fatalf("recovered pending replaced: %v", err)
	}
	// Evaluation cannot erase retained evidence even after a rejected proposal.
	if _, err := history.Reconcile([]network.EpochFacts{pending}); err != nil {
		t.Fatal(err)
	}
	conflicted, err := network.RestoreEpochHistory(current, pending, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conflicted.Reconcile([]network.EpochFacts{pending}); !errors.Is(err, network.ErrConflictedHistory) {
		t.Fatalf("sticky conflict lost: %v", err)
	}
}

func TestEpochHistoryRefusesRollbackBrokenChainAndFutureGenesis(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	genesis := network.EpochFacts{Network: [32]byte{1}, Digest: [32]byte{2}, Number: 1, ValidFrom: now.Add(time.Minute), ValidUntil: now.Add(time.Hour)}
	empty, err := network.RestoreEpochHistory(network.EpochFacts{}, network.EpochFacts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := empty.Reconcile([]network.EpochFacts{genesis})
	if err != nil {
		t.Fatal(err)
	}
	if pending, err := selection.DispositionAt(now); err != nil || pending != network.EpochDeferred {
		t.Fatal("future genesis became usable")
	}
	if pending, err := selection.DispositionAt(genesis.ValidFrom); err != nil || pending != network.EpochCurrent {
		t.Fatal("current genesis refused")
	}
	if _, err := network.RestoreEpochHistory(network.EpochFacts{}, genesis, false); err == nil {
		t.Fatal("pending without predecessor restored")
	}
	history, err := network.RestoreEpochHistory(genesis, network.EpochFacts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*network.EpochFacts){
		func(e *network.EpochFacts) { e.Number = 3 },
		func(e *network.EpochFacts) { e.Previous = [32]byte{9} },
		func(e *network.EpochFacts) { e.Network = [32]byte{9} },
	} {
		next := network.EpochFacts{Network: genesis.Network, Digest: [32]byte{3}, Previous: genesis.Digest, Number: 2, ValidFrom: genesis.ValidFrom, ValidUntil: genesis.ValidUntil}
		mutate(&next)
		if _, err := history.Reconcile([]network.EpochFacts{next}); !errors.Is(err, network.ErrEpochSequence) {
			t.Fatalf("invalid transition: %v", err)
		}
	}
	if _, err := (network.EpochSelection{}).DispositionAt(now); err == nil {
		t.Fatal("zero proposal accepted")
	}
}
