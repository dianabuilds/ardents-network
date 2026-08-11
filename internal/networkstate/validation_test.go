package networkstate_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

func TestOfflineValidationFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(*fixture, *networkstate.Config)
	}{
		{"epoch trailing bytes", func(value *fixture, _ *networkstate.Config) { value.epoch = append(value.epoch, 0) }},
		{"epoch signature", func(value *fixture, _ *networkstate.Config) { value.epoch[len(value.epoch)-1] ^= 0xff }},
		{"wrong network", func(_ *fixture, config *networkstate.Config) { config.NetworkID[0] ^= 0xff }},
		{"stale epoch", func(value *fixture, config *networkstate.Config) { config.Now = time.Unix(value.now+3600, 0) }},
		{"missing materialization", func(value *fixture, _ *networkstate.Config) { value.materializations = nil }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value := newFixture(t)
			root := t.TempDir()
			config := networkstate.Config{
				Root: root, NetworkID: value.networkID,
				Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
				Threshold:   1, Now: time.Unix(value.now, 0),
			}
			test.change(&value, &config)
			store, err := networkstate.Open(config)
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
	config := networkstate.Config{
		Root: root, NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0),
	}
	store, err := networkstate.Open(config)
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
	if _, err := networkstate.Open(config); err == nil {
		t.Fatal("corrupt current generation was recovered")
	}
}

func TestConfigRejectsUnboundedAutomaticAcquisition(t *testing.T) {
	t.Parallel()
	value := newFixture(t)
	base := networkstate.Config{Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic}, Threshold: 1,
		Now: time.Unix(value.now, 0)}
	tests := []struct {
		name string
		edit func(*networkstate.Config)
	}{
		{"materialization index", func(config *networkstate.Config) { config.SourceMaterializationIndex = 64 }},
		{"hot refresh", func(config *networkstate.Config) { config.AutomaticRefreshInterval = time.Nanosecond }},
		{"static observation", func(config *networkstate.Config) { config.AutomaticRefreshInterval = time.Second }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := base
			config.Root = t.TempDir()
			test.edit(&config)
			if store, err := networkstate.Open(config); err == nil {
				_ = store.Close()
				t.Fatal("invalid acquisition config was accepted")
			}
		})
	}
}
