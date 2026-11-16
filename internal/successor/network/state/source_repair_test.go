package state_test

import (
	"context"
	"errors"
	"testing"
	"time"

	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// defunctServingSource establishes the class-G recovery state: a real signed
// Source history whose serving invocation claimed the producer and then closed
// without a joined Finish, leaving the producer retained. The original is dead
// (its lease is released); only the producer obligation remains unresolved.
func defunctServingSource(t *testing.T) state2.Config {
	t.Helper()
	genesis := newFixture(t)
	config := installedServingSourceConfig(t, genesis)
	serving, err := state2.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	// Retain the command completion before Close, then never Finish: this is the
	// crash-before-joined-Finish state. Ordinary Close would remove the producer;
	// retained completion preserves it until Finish (which never runs here).
	if _, err := serving.RetainSourceCompletion(); err != nil {
		_ = serving.Close()
		t.Fatal(err)
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
	// The retained opening must refuse the unresolved producer before repair.
	if owner, err := state2.OpenRetained(config); err == nil {
		_ = owner.Close()
		t.Fatal("retained opening adopted an unresolved serving producer")
	}
	return config
}

func inactiveProof(context.Context) error { return nil }

func TestRepairRetainedSourceProducerClearsDefunctInvocationAndKeepsHistory(t *testing.T) {
	config := defunctServingSource(t)
	if err := state2.RepairRetainedSourceProducer(config, inactiveProof); err != nil {
		t.Fatalf("repair of a defunct serving invocation refused: %v", err)
	}
	// A new independently checked retained invocation now opens the same history.
	owner, err := state2.OpenRetained(config)
	if err != nil {
		t.Fatalf("retained relaunch after repair refused: %v", err)
	}
	if _, err := owner.Current(); err != nil {
		_ = owner.Close()
		t.Fatalf("repair lost the retained current generation: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	// The repair is idempotent-safe: a second repair finds no producer and refuses.
	if err := state2.RepairRetainedSourceProducer(config, inactiveProof); !errors.Is(err, state2.ErrSourceRepairUnavailable) {
		t.Fatalf("second repair = %v; want ErrSourceRepairUnavailable (no producer)", err)
	}
}

func TestRepairRetainedSourceProducerRefusesWithoutProcessDeathProof(t *testing.T) {
	config := defunctServingSource(t)
	refusal := errors.New("platform service still active")
	for _, proof := range []func(context.Context) error{
		nil,
		func(context.Context) error { return refusal },
	} {
		err := state2.RepairRetainedSourceProducer(config, proof)
		if err == nil {
			t.Fatal("repair cleared the producer without process-death proof")
		}
		if proof != nil && !errors.Is(err, refusal) {
			t.Fatalf("repair did not retain the original proof refusal: %v", err)
		}
		// The producer must remain retained; the bytes are untouched.
		if owner, err := state2.OpenRetained(config); err == nil {
			_ = owner.Close()
			t.Fatal("refused repair cleared the producer")
		}
	}
}

func TestRepairRetainedSourceProducerRefusesLiveLeaseHolder(t *testing.T) {
	genesis := newFixture(t)
	config := installedServingSourceConfig(t, genesis)
	live, err := state2.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	// A live serving invocation holds the exclusive lease; the repair must not
	// clear its producer (that would admit a concurrent invocation).
	if err := state2.RepairRetainedSourceProducer(config, inactiveProof); err == nil {
		t.Fatal("repair cleared the producer of a live serving invocation")
	}
}

func TestRepairRetainedSourceProducerRefusesCleanRoot(t *testing.T) {
	genesis := newFixture(t)
	config := installedServingSourceConfig(t, genesis)
	// A serving invocation that closes ordinarily (no retained completion) removes
	// its own producer: the participation root exists but no obligation remains.
	serving, err := state2.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := serving.Close(); err != nil {
		t.Fatal(err)
	}
	if err := state2.RepairRetainedSourceProducer(config, inactiveProof); !errors.Is(err, state2.ErrSourceRepairUnavailable) {
		t.Fatalf("repair of a clean root = %v; want ErrSourceRepairUnavailable", err)
	}
}

func TestRepairRetainedSourceProducerRefusesNonServingConfig(t *testing.T) {
	genesis := newFixture(t)
	config := fixtureConfig(genesis, t.TempDir(), time.Unix(genesis.now, 0).UTC())
	if err := state2.RepairRetainedSourceProducer(config, inactiveProof); !errors.Is(err, state2.ErrSourceRepairUnavailable) {
		t.Fatalf("repair of a non-serving config = %v; want ErrSourceRepairUnavailable", err)
	}
}
