package state_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// The deliberate G7 tightening (#772): configuration validation refuses
// relative and non-clean paths instead of silently absolutizing them through
// filepath.Abs. Every production caller already supplies absolute clean paths
// (Endpoint declarations, the command Admission/Source/Node plan validators),
// so no valid input relied on the former conversion.
func TestConfigRefusesRelativeAndNonCleanPaths(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := sha256.Sum256(public)
	base := func() state.Config {
		return state.Config{
			Root:                   t.TempDir(),
			Authorities:            map[[32]byte]ed25519.PublicKey{id: public},
			ClosedProfileAuthority: public,
			Threshold:              1,
			Now:                    time.Unix(1700000000, 0),
			ClockObservation:       time.Unix(1700000000, 0),
		}
	}
	for _, scenario := range []struct {
		name   string
		mutate func(*state.Config)
		prose  string
	}{
		{"relative root", func(c *state.Config) { c.Root = "relative-state" }, "state root must be an absolute clean path"},
		{"non-clean root", func(c *state.Config) { c.Root = t.TempDir() + string(filepath.Separator) }, "state root must be an absolute clean path"},
		{"relative clock observation", func(c *state.Config) { c.ClockObservationFile = "relative-clock" }, "clock observation file must be an absolute clean path"},
		{"non-clean clock observation", func(c *state.Config) { c.ClockObservationFile = t.TempDir() + string(filepath.Separator) }, "clock observation file must be an absolute clean path"},
		{"relative local role root", func(c *state.Config) { c.LocalRoleStateRoot = "relative-roles" }, "local role state root must be an absolute clean path"},
		{"non-clean local role root", func(c *state.Config) { c.LocalRoleStateRoot = t.TempDir() + string(filepath.Separator) }, "local role state root must be an absolute clean path"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			config := base()
			scenario.mutate(&config)
			owner, err := state.Open(config)
			if err == nil {
				owner.Close()
				t.Fatal("relative or non-clean path accepted")
			}
			if !strings.Contains(err.Error(), scenario.prose) {
				t.Fatalf("unexpected refusal: %v", err)
			}
		})
	}
	// Absolute clean paths remain accepted: the tightening changes no valid
	// caller's behavior.
	config := base()
	config.LocalRoleStateRoot = t.TempDir()
	observation := filepath.Join(t.TempDir(), "clock")
	if err := os.WriteFile(observation, nil, 0600); err != nil {
		t.Fatal(err)
	}
	config.ClockObservationFile = observation
	owner, err := state.Open(config)
	if err != nil {
		t.Fatalf("absolute clean configuration refused: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}
