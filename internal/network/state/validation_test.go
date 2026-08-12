package state_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestOfflineValidationFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*fixture, *state.Config)
	}{
		{"epoch trailing bytes", func(value *fixture, _ *state.Config) { value.epoch = append(value.epoch, 0) }},
		{"epoch signature", func(value *fixture, _ *state.Config) { value.epoch[len(value.epoch)-1] ^= 0xff }},
		{"wrong network", func(_ *fixture, config *state.Config) { config.NetworkID[0] ^= 0xff }},
		{"stale epoch", func(value *fixture, config *state.Config) { config.Now = time.Unix(value.now+3600, 0) }},
		{"missing materialization", func(value *fixture, _ *state.Config) { value.materializations = nil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newFixture(t)
			root := t.TempDir()
			config := state.Config{
				Root: root, NetworkID: value.networkID,
				Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
				Threshold:   1, Now: time.Unix(value.now, 0),
			}
			test.change(&value, &config)
			store, err := state.Open(config)
			if err != nil {
				t.Fatalf("open state: %v", err)
			}
			defer store.Close()
			if _, err := store.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err == nil {
				t.Fatal("invalid offline state was accepted")
			}
			if _, err := os.Stat(filepath.Join(root, "current")); !os.IsNotExist(err) {
				t.Fatalf("failed validation published current: %v", err)
			}
		})
	}
}

func TestOpenFailsOnCorruptCurrentGeneration(t *testing.T) {
	t.Parallel()
	value := newFixture(t)
	root := t.TempDir()
	config := state.Config{
		Root: root, NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0),
	}
	store, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Accept(context.Background(), value.epoch, value.inputs, value.materializations)
	if err != nil {
		t.Fatal(err)
	}
	epochPath := filepath.Join(root, "generations", snapshot.Generation, "epoch.bin")
	epoch, err := os.ReadFile(epochPath)
	if err != nil {
		t.Fatal(err)
	}
	epoch[0] ^= 0xff
	if err := os.WriteFile(epochPath, epoch, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := state.Open(config); err == nil {
		t.Fatal("corrupt current generation was recovered")
	}
}

func TestConfigRejectsUnboundedAutomaticAcquisition(t *testing.T) {
	t.Parallel()
	value := newFixture(t)
	base := state.Config{Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic}, Threshold: 1,
		Now: time.Unix(value.now, 0)}
	tests := []struct {
		name string
		edit func(*state.Config)
	}{
		{"materialization index", func(config *state.Config) { config.Source.MaterialIndex = 64 }},
		{"hot refresh", func(config *state.Config) { config.AutomaticRefreshInterval = time.Nanosecond }},
		{"static observation", func(config *state.Config) { config.AutomaticRefreshInterval = time.Second }},
		{"multiple clocks", func(config *state.Config) { config.Clock = time.Now }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Root = t.TempDir()
			test.edit(&config)
			if store, err := state.Open(config); err == nil {
				_ = store.Close()
				t.Fatal("invalid acquisition config was accepted")
			}
		})
	}
}
