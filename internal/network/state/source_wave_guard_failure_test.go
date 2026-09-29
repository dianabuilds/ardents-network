package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/source"
)

func TestFailedSourceGuardReleaseRetiresState(t *testing.T) {
	blockedRoot := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	state := &networkState{config: config{
		root: t.TempDir(), localRoles: blockedRoot, clock: time.Now,
		sourceInfo: source.Details{Configured: true, Identities: [2][32]byte{{1}, {2}}, Families: [2]string{"first", "second"}},
	}}
	state.distribution.cycleID = 1
	state.distribution.cycleDeadline = time.Now().Add(time.Hour).Unix()
	if err := state.releaseSourceWaveLocked(); err == nil || !strings.Contains(err.Error(), "release direct Source contact guard") {
		t.Fatalf("failed guard release returned %v", err)
	}
	if !state.closed || state.terminalErr == nil {
		t.Fatal("failed guard release left State available")
	}
}

func TestSourceWaveRefusesBeforeContactWhenDutyCapacityIsFull(t *testing.T) {
	now := time.Unix(1_800_000_100, 0).UTC()
	root := t.TempDir()
	roleRoot := filepath.Join(t.TempDir(), "roles")
	roles, err := duty.Open(duty.Config{Root: roleRoot, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	retained := make([]duty.Duty, 63)
	for index := range retained {
		retained[index] = duty.Duty{Identity: [32]byte{byte(index + 3)}, Family: [32]byte{byte(index + 3)},
			Class: "direct-source", State: "live", NotAfter: now.Add(time.Hour)}
	}
	if err := roles.Replace([32]byte{99}, retained); err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	storage, err := openTestDurableRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	state := &networkState{config: config{root: root, localRoles: roleRoot, clock: func() time.Time { return now },
		sourceInfo: source.Details{Configured: true, Identities: [2][32]byte{{1}, {2}}, Families: [2]string{"first", "second"}, OrderSeed: [32]byte{1}}},
		storage: storage}
	if _, _, err := state.startSourceWave(now); !errors.Is(err, duty.ErrInstallationSourceExhausted) {
		t.Fatalf("full duty capacity admitted the wave: %v", err)
	}
	if held, err := duty.ReadConflict(roleRoot, func() time.Time { return now }, [32]byte{1}, [32]byte{}); err != nil || held {
		t.Fatalf("failed wave installed an unbounded partial guard: held=%t err=%v", held, err)
	}
}
