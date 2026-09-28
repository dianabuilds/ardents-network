package state_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestSourceBootstrapWaitsForFutureGenesisWithoutStaging(t *testing.T) {
	genesis := newFixture(t)
	config, closeSources := sourceEnvironment(t, genesis, genesis, genesis)
	defer closeSources()
	root := t.TempDir()
	config.Root = root
	config.LocalRoleStateRoot = root + "-local-roles"
	now := time.Unix(fixtureNow-40, 0).UTC()
	config.Clock = func() time.Time { return now }
	config.ClockObservation = now

	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, refreshErr := store.Refresh(context.Background())
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, reopenErr := state.Open(config)
	if refreshErr == nil {
		if reopened != nil {
			_ = reopened.Close()
		}
		t.Fatalf("future genesis falsely accepted (epoch %d); reopen returned %v", snapshot.Epoch, reopenErr)
	}
	if !strings.Contains(refreshErr.Error(), "genesis Epoch is not yet current") {
		t.Fatalf("future genesis bootstrap returned %v", refreshErr)
	}
	if reopenErr != nil {
		t.Fatalf("reopen empty bootstrap root: %v", reopenErr)
	}
	if _, err := reopened.Current(); !errors.Is(err, state.ErrNoCurrentGeneration) {
		t.Fatalf("reopened future genesis became current: %v", err)
	}
	if _, err := reopened.Refresh(context.Background()); err == nil || !strings.Contains(err.Error(), "durable backoff") {
		t.Fatalf("future genesis retry was not bounded: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	now = time.Unix(fixtureNow-29, 0).UTC()
	config.ClockObservation = now
	current, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	snapshot, err = current.Refresh(context.Background())
	if err != nil || snapshot.Epoch != 1 || snapshot.Digest != genesis.epochDigest || snapshot.PendingEpoch != 0 {
		t.Fatalf("valid genesis bootstrap = %+v, %v", snapshot, err)
	}
}
