package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestCurrentNamespaceRequiresThresholdEpochAndMerkleMembership(t *testing.T) {
	t.Parallel()
	network := [32]byte{7}
	policy, signers := materializationPolicy("current-namespace", network)
	store, err := namespace.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	epoch := namespace.Epoch{Number: 11, Digest: [32]byte{11}, CutoffOffset: 500,
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: 3,
		RejectionRoot: sha256.Sum256([]byte("rejections")), RejectionLength: 1}
	if err := store.Commit(epoch, [][]byte{signedRecord(t, network, "alice", "authority-a")},
		thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	proof, err := store.Lookup("alice", 11)
	if err != nil {
		t.Fatal(err)
	}
	record, binding, warning, number, err := namespace.Verify(policy, proof, 11, epoch.Digest, 900_000)
	if err != nil || number != 11 || record.Name != "alice" || binding.Name != "alice" ||
		binding.Target != [32]byte{1} || binding.Commitment == [32]byte{} || warning != "" {
		t.Fatalf("verified=%+v binding=%+v warning=%q epoch=%d err=%v", record, binding, warning, number, err)
	}
	if _, _, _, _, err := namespace.Verify(policy, proof, 11, epoch.Digest, 950_000); err != nil {
		t.Fatalf("Record was unavailable at its exact validity boundary: %v", err)
	}
	if _, _, _, _, err := namespace.Verify(policy, proof, 11, epoch.Digest, 950_001); err == nil {
		t.Fatal("Record remained available after its signed validity boundary")
	}
	mutated := append([]byte(nil), proof...)
	mutated[len(mutated)-1] ^= 1
	if _, _, _, _, err := namespace.Verify(policy, mutated, 11, epoch.Digest, 900_000); err == nil {
		t.Fatal("mutated Namespace membership proof was accepted")
	}

	attackerPolicy, attacker := materializationPolicy("attacker-namespace", network)
	attackerStore, err := namespace.Open(t.TempDir(), attackerPolicy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = attackerStore.Close() })
	if err := attackerStore.Commit(epoch, [][]byte{signedRecord(t, network, "alice", "attacker-authority")},
		thresholdAttester(attacker[:2])); err != nil {
		t.Fatal(err)
	}
	attackerProof, err := attackerStore.Lookup("alice", 11)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := namespace.Verify(policy, attackerProof, 11, epoch.Digest, 900_000); err == nil {
		t.Fatal("self-consistent attacker Namespace was accepted under the installed epoch policy")
	}
}

func TestDeepestLegalNameHasCompactCurrentNamespaceProof(t *testing.T) {
	t.Parallel()
	network := [32]byte{8}
	policy, signers := materializationPolicy("deep-namespace", network)
	store, err := namespace.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	authoritySeed := sha256.Sum256([]byte("deep-name-authority"))
	authority := ed25519.NewKeyFromSeed(authoritySeed[:])
	records := make([][]byte, 127)
	for depth := 1; depth <= len(records); depth++ {
		name := strings.Repeat("a.", depth-1) + "a"
		record := namespace.Record{Name: name, Generation: 1, Revision: 1, Lease: "active",
			Consistency: "current", Recovery: "stable",
			Authority:      hex.EncodeToString(authority.Public().(ed25519.PublicKey)),
			LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000}
		if depth > 1 {
			record.ParentName = strings.Repeat("a.", depth-2) + "a"
			record.ParentGeneration = 1
		}
		if depth == len(records) {
			record.Target, record.RecordNotAfter = [32]byte{1}, 950_000
		}
		records[depth-1], err = namespace.SignRecord(network, record, authority)
		if err != nil {
			t.Fatal(err)
		}
	}
	epoch := namespace.Epoch{Number: 12, Digest: [32]byte{12}, CutoffOffset: 600,
		TransitionRoot: sha256.Sum256([]byte("deep-transitions")), TransitionLength: uint32(len(records)),
		RejectionRoot: sha256.Sum256([]byte("deep-rejections"))}
	if err := store.Commit(epoch, records, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("a.", 126) + "a"
	proof, err := store.Lookup(name, 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(proof) >= 3_900 {
		t.Fatalf("deep-name proof bytes=%d", len(proof))
	}
	record, binding, _, number, err := namespace.Verify(policy, proof, 12, epoch.Digest, 900_000)
	if err != nil || number != 12 || record.Name != name || binding.Target != [32]byte{1} {
		t.Fatalf("record=%+v binding=%+v epoch=%d err=%v", record, binding, number, err)
	}
}

func TestNamespaceTracerRejectsCorpusBeyondAcceptedEnvelope(t *testing.T) {
	t.Parallel()
	network := [32]byte{9}
	policy, signers := materializationPolicy("tracer-envelope", network)
	store, err := namespace.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	records := make([][]byte, 128)
	for index := range records {
		records[index] = signedRecord(t, network, fmt.Sprintf("tracer-%d", index), "tracer-authority")
	}
	epoch := namespace.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("tracer-transitions")), TransitionLength: uint32(len(records)),
		RejectionRoot: sha256.Sum256([]byte("tracer-rejections"))}
	if err := store.Commit(epoch, records, thresholdAttester(signers[:2])); err == nil {
		t.Fatal("corpus above the accepted 127-record tracer envelope was installed")
	}
}

func materializationPolicy(label string, network [32]byte) (namespace.MaterializationPolicy, []ed25519.PrivateKey) {
	policy := namespace.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1",
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	var signers []ed25519.PrivateKey
	for index := 0; index < 3; index++ {
		seed := sha256.Sum256([]byte(label + string(rune('0'+index))))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
		signers = append(signers, private)
	}
	return policy, signers
}

func thresholdAttester(signers []ed25519.PrivateKey) func([]byte) ([][32]byte, [][]byte, error) {
	return func(transcript []byte) ([][32]byte, [][]byte, error) {
		type signature struct {
			id  [32]byte
			raw []byte
		}
		values := make([]signature, len(signers))
		for index, private := range signers {
			values[index] = signature{id: sha256.Sum256(private.Public().(ed25519.PublicKey)),
				raw: ed25519.Sign(private, transcript)}
		}
		sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
		ids, signatures := make([][32]byte, len(values)), make([][]byte, len(values))
		for index, value := range values {
			ids[index], signatures[index] = value.id, value.raw
		}
		return ids, signatures, nil
	}
}
