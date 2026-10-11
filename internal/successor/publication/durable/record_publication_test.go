package durable

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// Fixture issuance/signing supplies independent public bytes. It does not
// represent a host binding, live Publisher or successful registration/ACK.
func generationRecord(credential publication.Credential, acknowledgement byte) []byte {
	value := credential.Delegation()
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, 32))
	instance := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{34}, 32))
	raw := append([]byte{0, 3}, value.Authority[:]...)
	raw = append(raw, value.Target[:]...)
	raw = append(raw, value.Instance[:]...)
	for _, number := range []uint64{value.Generation, uint64(value.NotBefore.Unix()), uint64(value.NotAfter.Unix())} {
		raw = binary.BigEndian.AppendUint64(raw, number)
	}
	raw = append(raw, value.Network[:]...)
	raw = binary.BigEndian.AppendUint32(raw, 3)
	raw = append(raw, ed25519.Sign(authority, raw)...)
	record := append([]byte("ardents-service-publication-v3\x00"), raw...)
	record = append(record, bytes.Repeat([]byte{acknowledgement}, 32)...)
	digest := sha256.Sum256(record)
	return append(record, ed25519.Sign(instance, digest[:])...)
}

func TestGenerationRecordExactRetryAndJoinedRetirement(t *testing.T) {
	credential := generationCredential(t, 7)
	value := credential.Delegation()
	config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network}
	root, err := openWith(t.Context(), config, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	generation, err := root.Reserve(t.Context(), credential)
	if err != nil {
		t.Fatal(err)
	}
	defer generation.Close()
	raw := generationRecord(credential, 68)
	expected := bytes.Clone(raw)
	if err := generation.Publish(t.Context(), raw); err == nil {
		t.Fatal("record published before original durable consumption")
	}
	if err := generation.Advance(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Mutate the caller's buffer after verification and before record output.
	// The durable owner must retain its own exact immutable input.
	count := 0
	root.flush = func(string) error {
		count++
		if count == 1 {
			clear(raw)
		}
		return nil
	}
	if err := generation.Publish(t.Context(), raw); err != nil {
		t.Fatal(err)
	}
	raw = expected
	if count != 4 {
		t.Fatal("publication did not complete all four directory barriers", count)
	}
	paths := []string{filepath.Join(config.Root, "current"), filepath.Join(config.Root, "generations", "0000000000000007", "publication.bin")}
	stored, err := os.ReadFile(paths[1])
	if err != nil || !bytes.Equal(stored, expected) {
		t.Fatal("caller mutation changed immutable persisted record", err)
	}
	original := make([]os.FileInfo, len(paths))
	for i, path := range paths {
		original[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := generation.Publish(t.Context(), generationRecord(credential, 69)); err == nil {
		t.Fatal("changed signed record replaced immutable generation")
	}
	if err := generation.Publish(t.Context(), raw); err != nil {
		t.Fatal(err)
	}
	for i, path := range paths {
		retry, err := os.Stat(path)
		if err != nil || !os.SameFile(original[i], retry) {
			t.Fatal("exact retry rewrote original record/pointer", err)
		}
	}
	if err := generation.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("joined retirement retained publication bytes", err)
		}
	}
	if floor, err := root.Floor(t.Context()); err != nil || floor != 7 {
		t.Fatal("retirement rolled back original floor", floor, err)
	}
}

func TestGenerationRecordPostEffectFailuresReconcileWithoutAuthority(t *testing.T) {
	for _, mode := range []string{"flush-failure", "caller-loss"} {
		for phase := 1; phase <= 4; phase++ {
			t.Run(mode+string(rune('0'+phase)), func(t *testing.T) {
				credential := generationCredential(t, 7)
				value := credential.Delegation()
				config := Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network}
				root, err := openWith(t.Context(), config, func(string) error { return nil })
				if err != nil {
					t.Fatal(err)
				}
				generation, err := root.Reserve(t.Context(), credential)
				if err != nil {
					root.Close()
					t.Fatal(err)
				}
				original := root
				t.Cleanup(func() {
					generation.Close()
					original.Close()
				})
				if err := generation.Advance(t.Context()); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				count := 0
				failure := errors.New("controlled record directory barrier failure")
				root.flush = func(string) error {
					count++
					if count != phase {
						return nil
					}
					if mode == "caller-loss" {
						cancel()
						return nil
					}
					return failure
				}
				raw := generationRecord(credential, 68)
				if err := generation.Publish(ctx, raw); err == nil {
					t.Fatal("post-effect failure granted immutable publication receipt")
				}
				if err := generation.Publish(t.Context(), raw); err == nil {
					t.Fatal("ambiguous owner granted a retry receipt")
				}
				if _, err := root.Floor(t.Context()); err == nil {
					t.Fatal("ambiguous owner granted a floor receipt")
				}
				generation.Close()
				first := root.Close()
				if first == nil || root.Close() != first {
					t.Fatal("first physical failure lost at root release")
				}
				root, err = openWith(t.Context(), config, func(string) error { return nil })
				if err != nil {
					t.Fatal("owned exact residue did not reconcile", err)
				}
				defer root.Close()
				if floor, err := root.Floor(t.Context()); err != nil || floor != 7 {
					t.Fatal("reconciliation rolled back spent floor", floor, err)
				}
				if _, err := root.Reserve(t.Context(), credential); !errors.Is(err, ErrConsumed) {
					t.Fatal("reopen revived consumed authority", err)
				}
			})
		}
	}
}
