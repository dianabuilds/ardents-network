package durable

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// Independent fixture issuance supplies public inputs, not live host authority.
func generationCredential(t *testing.T, number uint64) publication.Credential {
	t.Helper()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, 32))
	var public [32]byte
	copy(public[:], authority[32:])
	target, network := publication.Target(public), [32]byte{51}
	before := time.Now().UTC().Truncate(time.Second)
	raw := append([]byte{0, 3}, public[:]...)
	raw = append(raw, target[:]...)
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{34}, 32))
	raw = append(raw, instance[32:]...)
	for _, value := range []uint64{number, uint64(before.Unix()), uint64(before.Add(time.Hour).Unix())} {
		raw = binary.BigEndian.AppendUint64(raw, value)
	}
	raw = append(raw, network[:]...)
	raw = binary.BigEndian.AppendUint32(raw, 3)
	raw = append(raw, ed25519.Sign(authority, raw)...)
	credential, err := publication.VerifyCredential(raw, target, network, before)
	if err != nil {
		t.Fatal(err)
	}
	return credential
}

func TestReservedGenerationMonotonicAndExactRetry(t *testing.T) {
	credential := generationCredential(t, ^uint64(0))
	value := credential.Delegation()
	config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network}
	root, err := openWith(context.Background(), config, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	generation, err := root.Reserve(context.Background(), credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.Reserve(context.Background(), credential); err == nil {
		t.Fatal("retained reservation was duplicated")
	}
	if err = generation.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, err := os.Stat(filepath.Join(config.Root, "floor"))
	if err != nil {
		t.Fatal(err)
	}
	if err = generation.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(filepath.Join(config.Root, "floor"))
	if err != nil || !os.SameFile(first, second) {
		t.Fatal("exact retry rewrote the original floor", err)
	}
	raw, err := os.ReadFile(filepath.Join(config.Root, "floor"))
	if err != nil || string(raw) != "18446744073709551615\n" {
		t.Fatal("full uint64 floor lost", err)
	}
	generation.Close()
	if _, err = root.Reserve(context.Background(), credential); !errors.Is(err, ErrConsumed) {
		t.Fatal("spent generation reacquired", err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFloorPostRenameFailureDeniesReceiptsUntilReopen(t *testing.T) {
	for _, mode := range []string{"flush-failure", "caller-loss"} {
		t.Run(mode, func(t *testing.T) {
			credential := generationCredential(t, 7)
			value := credential.Delegation()
			config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network}
			root, err := openWith(context.Background(), config, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			generation, err := root.Reserve(ctx, credential)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("controlled directory flush failure")
			// This fixture controls only the final mechanism barrier. Native
			// Instance tests separately run actual file and directory durability.
			root.flush = func(string) error {
				if mode == "caller-loss" {
					cancel()
					return nil
				}
				return failure
			}
			if err = generation.Advance(ctx); err == nil {
				t.Fatal("post-rename failure returned durable success")
			}
			if _, err = root.Floor(context.Background()); err == nil {
				t.Fatal("ambiguous owner supplied a floor receipt")
			}
			if err = generation.Advance(context.Background()); err == nil {
				t.Fatal("ambiguous owner retried authority")
			}
			raw, err := os.ReadFile(filepath.Join(config.Root, "floor"))
			if err != nil || string(raw) != "7\n" {
				t.Fatal("visible floor was rolled back", err)
			}
			generation.Close()
			first := root.Close()
			if first == nil || root.Close() != first {
				t.Fatal("first failure did not survive Close")
			}
			root, err = openWith(context.Background(), config, func(string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			if floor, err := root.Floor(context.Background()); err != nil || floor != 7 {
				t.Fatal("reopen failed to reconcile visible floor", floor, err)
			}
		})
	}
}
