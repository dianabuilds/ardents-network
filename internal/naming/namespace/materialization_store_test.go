package namespace_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestStoreSurvivesRestartAndRejectsStaleEpoch(t *testing.T) {
	t.Parallel()
	root, network := t.TempDir(), [32]byte{7}
	policy, signers := materializationPolicy("restart", network)
	store, err := namespace.Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	first := signedRecord(t, network, "alice", "authority-a")
	second := signedRecord(t, network, "bob", "authority-b")
	if err := store.Commit(testEpoch(11), [][]byte{first, second}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := namespace.Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	proof, proofErr := reopened.Lookup("alice", 11)
	record, _, _, _, verifyErr := namespace.Verify(policy, proof, 11, [32]byte{11}, 900_000)
	if proofErr != nil || verifyErr != nil || record.Name != "alice" {
		t.Fatalf("record=%+v err=%v/%v", record, proofErr, verifyErr)
	}
	if _, err := reopened.Lookup("alice", 12); err == nil || err.Error() != "naming state is stale" {
		t.Fatalf("stale epoch err=%v", err)
	}
}

func TestStoreRejectsTamperAndPartialBatch(t *testing.T) {
	t.Parallel()
	root, network := t.TempDir(), [32]byte{8}
	policy, signers := materializationPolicy("tamper", network)
	store, err := namespace.Open(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	valid := signedRecord(t, network, "alice", "authority-a")
	if err := store.Commit(testEpoch(3), [][]byte{valid}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(testEpoch(4), [][]byte{valid, []byte{1}}, thresholdAttester(signers[:2])); err == nil {
		t.Fatal("partial invalid batch committed")
	}
	if _, err := store.Lookup("alice", 3); err != nil {
		t.Fatalf("previous batch changed: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "generations", "*", "inputs", "0000.bin"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("generation paths=%v err=%v", paths, err)
	}
	raw, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	if err := os.WriteFile(paths[0], raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Lookup("alice", 3); err == nil || err.Error() != "naming state is tampered" {
		t.Fatalf("tamper err=%v", err)
	}
}

func testEpoch(number uint64) namespace.Epoch {
	return namespace.Epoch{Number: number, CutoffOffset: int64(number),
		Digest:         [32]byte{byte(number)},
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: 1,
		RejectionRoot: sha256.Sum256([]byte("rejections"))}
}

func signedRecord(t *testing.T, network [32]byte, name, label string) []byte {
	t.Helper()
	seed := sha256.Sum256([]byte(label))
	private := ed25519.NewKeyFromSeed(seed[:])
	record := namespace.Record{Name: name, Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(private.Public().(ed25519.PublicKey)), Target: [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, RecordNotAfter: 950_000, Continuity: 1}
	signed, err := namespace.SignRecord(network, record, private)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
