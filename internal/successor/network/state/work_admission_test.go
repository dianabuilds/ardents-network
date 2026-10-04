package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestExternalWorkRefusalPrecedesNetworkRootEffects(t *testing.T) {
	value := newFixture(t)
	root := filepath.Join(t.TempDir(), "state")
	config := fixtureConfig(value, root, time.Unix(value.now, 0))
	failure := errors.New("external application admission unavailable")
	config.PermitWork = func() error { return failure }
	owner, err := state.Open(config)
	if err == nil {
		_ = owner.Close()
		t.Fatal("Network ignored application work refusal")
	}
	if !errors.Is(err, failure) {
		t.Fatal("lost external refusal", err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refusal touched root", err)
	}
}

func TestExternalWorkRefusalStopsOpenedIntakeAndAllowsClose(t *testing.T) {
	value := newFixture(t)
	root := t.TempDir()
	config := fixtureConfig(value, root, time.Unix(value.now, 0))
	failure := errors.New("application stopped Network admission")
	var refused atomic.Bool
	config.PermitWork = func() error {
		if refused.Load() {
			return failure
		}
		return nil
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	refused.Store(true)
	_, epochErr := owner.Accept(t.Context(), value.epoch, value.inputs, value.materializations)
	_, profileErr := owner.AcceptClosedProfile(nil)
	_, runtimeErr := owner.CurrentRuntime()
	_, refreshErr := owner.Refresh(t.Context())
	for _, err := range []error{epochErr, profileErr, runtimeErr, refreshErr} {
		if !errors.Is(err, failure) {
			t.Fatalf("lost application admission refusal: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "current")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused intake published a generation: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("external refusal prevented joined close: %v", err)
	}
}
