package state_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestProductAcceptsFrozenOfflineVector(t *testing.T) {
	t.Parallel()
	base := filepath.Join("..", "..", "..", "tests", "e2e", "network-state", "testdata")
	epoch := readGoldenHex(t, filepath.Join(base, "epoch.hex"))
	inputs := make([][]byte, 8)
	for index := range inputs {
		inputs[index] = readGoldenHex(t, filepath.Join(base, fmt.Sprintf("input-%04d.hex", index)))
	}
	material := readGoldenHex(t, filepath.Join(base, "materialization-0000.hex"))
	networkID := decodeGoldenArray(t, "488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d")
	public := ed25519.PublicKey(readGoldenString(t, "c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631"))
	authorityID := sha256.Sum256(public)
	store, err := state.Open(state.Config{
		Root: t.TempDir(), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{authorityID: public},
		Threshold:   1, Now: time.Unix(1_800_000_100, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snapshot, err := store.Accept(context.Background(), epoch, inputs, [][]byte{material})
	if err != nil {
		t.Fatalf("accept frozen vector: %v", err)
	}
	if snapshot.Generation != "243fba444fe71948f6cd4a253552301192857a156c7eb6359eed604c2d2cda4b" {
		t.Fatalf("generation = %s", snapshot.Generation)
	}
}

func readGoldenHex(t *testing.T, path string) []byte {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(string(encoded[:len(encoded)-1]))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func readGoldenString(t *testing.T, encoded string) []byte {
	t.Helper()
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func decodeGoldenArray(t *testing.T, encoded string) [32]byte {
	t.Helper()
	var result [32]byte
	copy(result[:], readGoldenString(t, encoded))
	return result
}
