package state_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	statequalification "github.com/dianabuilds/ardents-network/internal/qualification/state"
)

type frozenFixture struct {
	root             string
	networkID        [32]byte
	authorityID      [32]byte
	authorityPublic  ed25519.PublicKey
	generation       string
	now              time.Time
	materializations [][]byte
}

func TestVerifierAcceptsFrozenOfflineVector(t *testing.T) {
	t.Parallel()
	fixture := mountFrozenFixture(t)
	result := verifyFrozenFixture(fixture)
	if result.Verdict != "pass" {
		t.Fatalf("golden verdict = %q (%s)", result.Verdict, result.Reason)
	}
}

func mountFrozenFixture(t *testing.T) frozenFixture {
	t.Helper()
	base := filepath.Join("..", "..", "..", "tests", "qualification", "h3-state-v1", "testdata")
	manifestBytes, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema             string   `json:"schema"`
		NetworkID          string   `json:"network_id"`
		AuthorityPublic    string   `json:"authority_public"`
		Threshold          int      `json:"threshold"`
		VerificationUnix   int64    `json:"verification_unix"`
		InputCount         int      `json:"input_count"`
		ExpectedGeneration string   `json:"expected_generation"`
		RejectionCodes     []uint16 `json:"expected_rejection_codes"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("parse frozen manifest: %v", err)
	}
	if manifest.Schema != "ardents-h3-state-manifest-v1" || manifest.InputCount != 8 || len(manifest.RejectionCodes) != 6 {
		t.Fatalf("unexpected frozen manifest: %+v", manifest)
	}
	root := t.TempDir()
	inputsDirectory := filepath.Join(root, "generations", manifest.ExpectedGeneration, "inputs")
	if err := os.MkdirAll(inputsDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	writeGoldenBinary(t, filepath.Join(root, "generations", manifest.ExpectedGeneration, "epoch.bin"), filepath.Join(base, "epoch.hex"))
	for index := range manifest.InputCount {
		writeGoldenBinary(t, filepath.Join(inputsDirectory, fmt.Sprintf("%04d.bin", index)),
			filepath.Join(base, fmt.Sprintf("input-%04d.hex", index)))
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(manifest.ExpectedGeneration+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	public := ed25519.PublicKey(goldenBytes(t, manifest.AuthorityPublic))
	return frozenFixture{root: root, networkID: goldenArray(t, manifest.NetworkID), authorityID: sha256.Sum256(public),
		authorityPublic: public, generation: manifest.ExpectedGeneration, now: time.Unix(manifest.VerificationUnix, 0),
		materializations: [][]byte{readGoldenBinary(t, filepath.Join(base, "materialization-0000.hex"))}}
}

func verifyFrozenFixture(fixture frozenFixture) statequalification.Result {
	return statequalification.Verify(statequalification.Case{Root: fixture.root, NetworkID: fixture.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{fixture.authorityID: fixture.authorityPublic}, Threshold: 1,
		Now: fixture.now, Materializations: fixture.materializations})
}

func writeGoldenBinary(t *testing.T, destination, source string) {
	t.Helper()
	if err := os.WriteFile(destination, readGoldenBinary(t, source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readGoldenBinary(t *testing.T, path string) []byte {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return goldenBytes(t, string(encoded[:len(encoded)-1]))
}

func goldenBytes(t *testing.T, encoded string) []byte {
	t.Helper()
	value, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func goldenArray(t *testing.T, encoded string) [32]byte {
	t.Helper()
	var value [32]byte
	copy(value[:], goldenBytes(t, encoded))
	return value
}
