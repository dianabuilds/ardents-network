package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

func TestSourcePlanRotationRefusesRecoveryWithUnprovenExposureHistory(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()

	first, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := first.Refresh(context.Background())
	if err != nil {
		_ = first.Close()
		t.Fatalf("first Source wave failed: %v", err)
	}
	if accepted.Epoch != 2 || accepted.SourceAttempts != 2 {
		_ = first.Close()
		t.Fatalf("first Source wave epoch=%d attempts=%d", accepted.Epoch, accepted.SourceAttempts)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	controlPath := filepath.Join(config.Root, "distribution", "current")
	before, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}

	original := config.Source.Sources[0].Identity
	config.Source.Sources[0].Identity = sha256.Sum256([]byte("replacement-source-identity"))
	rotated, reopenErr := state.OpenRetained(config)
	if rotated != nil {
		_ = rotated.Close()
	}
	var required *state.RecoveryRequiredError
	if !errors.As(reopenErr, &required) || !strings.Contains(reopenErr.Error(), "Source exposure history") {
		t.Fatalf("changed Source plan recovered without proven guards: %v", reopenErr)
	}
	withoutSources := fixtureConfig(genesis, config.Root, time.Unix(genesis.now, 0).UTC())
	observer, err := state.OpenRetained(withoutSources)
	if err != nil {
		t.Fatalf("terminal Source journal refused offline reader: %v", err)
	}
	if _, err := observer.Current(); err != nil {
		t.Fatalf("offline reader lost accepted State: %v", err)
	}
	if err := observer.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("exhausted Source plan changed the distribution pointer")
	}
	config.Source.Sources[0].Identity = original
	reopened, reopenErr := state.OpenRetained(config)
	if reopenErr != nil {
		t.Fatalf("reopen with original Source plan: %v", reopenErr)
	}
	defer reopened.Close()
	current, err := reopened.Current()
	if err != nil {
		t.Fatalf("retained State: %v", err)
	}
	if current.Epoch != 2 || current.SourceAttempts != 2 {
		t.Fatalf("retained State epoch=%d attempts=%d", current.Epoch, current.SourceAttempts)
	}
}
