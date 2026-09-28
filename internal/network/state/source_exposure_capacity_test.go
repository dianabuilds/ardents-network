package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestSourcePlanRotationRefusesBeforeExhaustingDurableHistory(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	defer closeSources()

	first, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := first.Refresh(context.Background())
	if err != nil || accepted.Epoch != 2 || accepted.SourceAttempts != 2 {
		_ = first.Close()
		t.Fatalf("first Source wave epoch=%d attempts=%d err=%v", accepted.Epoch, accepted.SourceAttempts, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	controlPath := filepath.Join(config.Root, "distribution", "current")
	before, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}

	config.Source.Sources[0].Identity = sha256.Sum256([]byte("replacement-source-identity"))
	rotated, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, refreshErr := rotated.Refresh(context.Background())
	if err := rotated.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, reopenErr := state.Open(config)
	if refreshErr == nil {
		if reopened != nil {
			_ = reopened.Close()
		}
		t.Fatalf("third Source exposure was accepted; reopen returned %v", reopenErr)
	}
	if !strings.Contains(refreshErr.Error(), "Source exposure history is full") {
		t.Fatalf("Source exposure exhaustion returned %v", refreshErr)
	}
	after, err := os.ReadFile(controlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("exhausted Source plan changed the distribution pointer")
	}
	if reopenErr != nil {
		t.Fatalf("reopen after refused Source plan: %v", reopenErr)
	}
	defer reopened.Close()
	current, err := reopened.Current()
	if err != nil || current.Epoch != 2 || current.SourceAttempts != 2 {
		t.Fatalf("retained State epoch=%d attempts=%d err=%v", current.Epoch, current.SourceAttempts, err)
	}
}
