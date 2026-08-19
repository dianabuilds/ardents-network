//go:build linux && live

package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func runBlockedG4PhaseProcess(t *testing.T) {
	phase := os.Getenv("ARDENTS_G4_PHASE")
	if phase == "reopen" {
		runBlockedG4Reopen(t)
		return
	}
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	if phase == "after-import" {
		publishBlockedG4Checkpoint(t, phase)
	}
	transition, manifest := blockedG4Transition(t)
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	defer func() { _ = closeOwner() }()
	if phase == "during-cleanup" {
		channel, cleanup, err := owner.Acquire(context.Background(), transition, manifest, time.Now().Add(time.Minute),
			func(context.Context, [32]byte, []byte, time.Time) (net.Conn, func() error, bool, error) {
				left, right := net.Pipe()
				_ = right.Close()
				return left, func() error { publishBlockedG4Checkpoint(t, phase); return nil }, true, nil
			})
		if err != nil || channel == nil || cleanup == nil {
			t.Fatalf("start G4 cleanup phase: %v", err)
		}
		_ = cleanup()
		t.Fatal("G4 cleanup checkpoint returned")
	}
	_, _, ordinal, started, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	contacts := map[string]int{"after-regime-publication": 1, "after-exposure-0": 1,
		"after-exposure-1": 2, "after-exposure-2": 3, "after-exposure-3": 4}[phase]
	if contacts > 0 {
		for count := 1; count < contacts; count++ {
			if err := owner.FinishContact(ordinal, started+uint64(count)*uint64(time.Second), false, true); err != nil {
				t.Fatal(err)
			}
			_, _, ordinal, err = owner.NextContact(context.Background())
			if err != nil {
				t.Fatal(err)
			}
		}
		publishBlockedG4Checkpoint(t, phase)
	}
	if phase == "after-terminal-record" {
		if err := owner.FinishContact(ordinal, started+1, true, true); err != nil {
			t.Fatal(err)
		}
		publishBlockedG4Checkpoint(t, phase)
	}
	t.Fatalf("unsupported G4 phase %q", phase)
}

func blockedG4Transition(t *testing.T) ([]byte, [32]byte) {
	t.Helper()
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	timeline := startBlockedTimeline(t)
	time.Sleep(time.Millisecond)
	transition = stampBlockedTransition(t, transition, timeline)
	var entry struct {
		RouteManifestDigest string `json:"route_manifest_digest"`
	}
	if err := planfile.Decode("/run/secure/entry.json", 32<<10, &entry); err != nil {
		t.Fatal(err)
	}
	var manifest [32]byte
	if err := planfile.FixedHex(entry.RouteManifestDigest, manifest[:]); err != nil {
		t.Fatal(err)
	}
	return transition, manifest
}

func publishBlockedG4Checkpoint(t *testing.T, phase string) {
	t.Helper()
	snapshot, present, err := readBlockedRestartPhase("/run/state/bridge")
	if err != nil || !present {
		t.Fatalf("read G4 checkpoint: present=%t err=%v", present, err)
	}
	writeBlockedJSON(t, filepath.Join(blockedSync(), "g4-checkpoint.json"),
		blockedG4Checkpoint{Kind: "g4-checkpoint", Phase: phase, Snapshot: snapshot})
	for {
		time.Sleep(time.Hour)
	}
}

func runBlockedG4Reopen(t *testing.T) {
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	defer func() { _ = closeOwner() }()
	evidence, err := owner.Evidence()
	result := blockedG4ReopenResult{Kind: "g4-reopen", Phase: os.Getenv("ARDENTS_G4_REOPENED_PHASE")}
	if err == nil {
		result.Attempt, result.Contacts, result.Terminal = true, int(evidence.ContactStarts), evidence.Terminal
	}
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}
