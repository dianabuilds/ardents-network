package state_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func TestOfflineStateRefusesPreCanceledCallerBeforeRootBirth(t *testing.T) {
	fixture := newFixture(t)
	root := filepath.Join(t.TempDir(), "state")
	config := fixtureConfig(fixture, root, time.Unix(fixture.now, 0))
	caller, cancel := context.WithCancel(t.Context())
	cancel()

	if result := state.RunOffline(caller, "accept-epoch", config, fixture.epoch, fixture.inputs, fixture.materializations, nil); result.Outcome != "canceled" {
		t.Fatalf("pre-canceled signed intake returned %q", result.Outcome)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("already-canceled signed intake created a State root: %v", err)
	}
	if result := state.RunOffline(t.Context(), "accept-epoch", config, fixture.epoch, fixture.inputs, fixture.materializations, nil); result.Outcome != "accepted-epoch" {
		t.Fatalf("live signed intake returned %q", result.Outcome)
	}
}

func TestOfflineStateCancellationAfterOpenRetainsRootAndReleasesLease(t *testing.T) {
	fixture := newFixture(t)
	root := filepath.Join(t.TempDir(), "state")
	config := fixtureConfig(fixture, root, time.Unix(fixture.now, 0))
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	config.PermitWork = func() error {
		_, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		cancel()
		return nil
	}
	if result := state.RunOffline(caller, "accept-epoch", config, fixture.epoch, fixture.inputs, fixture.materializations, nil); result.Outcome != "canceled" {
		t.Fatalf("intake canceled after opening returned %q", result.Outcome)
	}
	if info, err := os.Lstat(root); err != nil || !info.IsDir() {
		t.Fatalf("original State root did not survive cancellation: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "current")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled intake committed a generation: %v", err)
	}
	config.PermitWork = nil
	if result := state.RunOffline(t.Context(), "accept-epoch", config, fixture.epoch, fixture.inputs, fixture.materializations, nil); result.Outcome != "accepted-epoch" {
		t.Fatalf("signed intake through reopened root returned %q", result.Outcome)
	}
}
