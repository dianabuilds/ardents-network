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

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

func TestCurrentNamespaceRequiresThresholdEpochAndMerkleMembership(t *testing.T) {
	t.Parallel()
	network := [32]byte{7}
	policy, signers := materializationPolicy("current-namespace", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	currentEpoch := epoch.Epoch{Number: 11, Digest: [32]byte{11}, CutoffOffset: 500,
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: 3,
		RejectionRoot: sha256.Sum256([]byte("rejections")), RejectionLength: 1}
	if err := store.CommitLegacy(currentEpoch, [][]byte{signedRecord(t, network, "alice", "authority-a")},
		thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	proof, err := store.Lookup("alice", 11)
	if err != nil {
		t.Fatal(err)
	}
	binding, warning, number, err := epoch.VerifyBinding(policy, proof, 11, currentEpoch.Digest, 900_000)
	if err != nil || number != 11 || binding.Name != "alice" ||
		binding.Target != [32]byte{1} || binding.Commitment == [32]byte{} || warning != "" {
		t.Fatalf("binding=%+v warning=%q epoch=%d err=%v", binding, warning, number, err)
	}
	if _, _, _, err := epoch.VerifyBinding(policy, proof, 11, currentEpoch.Digest, 950_000); err != nil {
		t.Fatalf("Record was unavailable at its exact validity boundary: %v", err)
	}
	if _, _, _, err := epoch.VerifyBinding(policy, proof, 11, currentEpoch.Digest, 950_001); err == nil {
		t.Fatal("Record remained available after its signed validity boundary")
	}
	mutated := append([]byte(nil), proof...)
	mutated[len(mutated)-1] ^= 1
	if _, _, _, err := epoch.VerifyBinding(policy, mutated, 11, currentEpoch.Digest, 900_000); err == nil {
		t.Fatal("mutated Namespace membership proof was accepted")
	}

	attackerPolicy, attacker := materializationPolicy("attacker-namespace", network)
	attackerStore, err := epoch.Open(t.TempDir(), attackerPolicy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = attackerStore.Close() })
	if err := attackerStore.CommitLegacy(currentEpoch, [][]byte{signedRecord(t, network, "alice", "attacker-authority")},
		thresholdAttester(attacker[:2])); err != nil {
		t.Fatal(err)
	}
	attackerProof, err := attackerStore.Lookup("alice", 11)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := epoch.VerifyBinding(policy, attackerProof, 11, currentEpoch.Digest, 900_000); err == nil {
		t.Fatal("self-consistent attacker Namespace was accepted under the installed epoch policy")
	}
}

func TestCurrentNamespaceProofDerivesGraceFromSignedDeadline(t *testing.T) {
	t.Parallel()
	network := [32]byte{10}
	policy, signers := materializationPolicy("grace-materialization", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seed := sha256.Sum256([]byte("grace-materialization-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	active := record.Record{Name: "alice", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable",
		Authority:      hex.EncodeToString(private.Public().(ed25519.PublicKey)),
		Target:         [32]byte{1},
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, RecordNotAfter: 2_000_000, Continuity: 1}
	signedActive, err := record.SignRecord(network, active, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitLegacy(testEpoch(11), [][]byte{signedActive}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	activeProof, err := store.Lookup("alice", 11)
	if err != nil {
		t.Fatal(err)
	}
	binding, warning, number, err := epoch.VerifyBinding(policy, activeProof, 11, [32]byte{11}, 1_001_000)
	if err != nil || number != 11 || binding.Revision != 1 ||
		warning != "name lineage is in grace and should be treated as volatile" {
		t.Fatalf("derived grace binding=%+v warning=%q epoch=%d err=%v", binding, warning, number, err)
	}

	grace := active
	grace.Revision, grace.Lease = 2, "grace"
	signedGrace, err := record.SignRecord(network, grace, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitLegacy(testEpoch(12), [][]byte{signedGrace}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	graceProof, err := store.Lookup("alice", 12)
	if err != nil {
		t.Fatal(err)
	}
	binding, warning, number, err = epoch.VerifyBinding(policy, graceProof, 12, [32]byte{12}, 1_001_000)
	if err != nil || number != 12 || binding.Revision != 2 ||
		warning != "name lineage is in grace and should be treated as volatile" {
		t.Fatalf("grace binding=%+v warning=%q epoch=%d err=%v", binding, warning, number, err)
	}
	if _, _, _, err := epoch.VerifyBinding(policy, graceProof, 12, [32]byte{12}, 2_000_001); err == nil {
		t.Fatal("grace proof resolved after its signed Grace boundary")
	}

	released := grace
	released.Revision, released.Lease = 3, "released"
	released.LeaseExpiresAt, released.GraceExpiresAt = 0, 0
	signedReleased, err := record.SignRecord(network, released, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitLegacy(testEpoch(13), [][]byte{signedReleased}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	releasedProof, err := store.Lookup("alice", 13)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := epoch.VerifyBinding(policy, releasedProof, 13, [32]byte{13}, 1_500_000); err == nil {
		t.Fatal("released Name proof resolved")
	}
}

func TestCurrentNamespaceProofDerivesGraceFromParentDeadline(t *testing.T) {
	t.Parallel()
	network := [32]byte{14}
	policy, signers := materializationPolicy("parent-grace-materialization", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seed := sha256.Sum256([]byte("parent-grace-materialization-authority"))
	private := ed25519.NewKeyFromSeed(seed[:])
	authority := hex.EncodeToString(private.Public().(ed25519.PublicKey))
	parent := record.Record{Name: "root", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable", Authority: authority,
		LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
	child := record.Record{Name: "site.root", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable", Authority: authority, Target: [32]byte{1},
		ParentName: "root", ParentGeneration: 1,
		LeaseExpiresAt: 3_000, GraceExpiresAt: 4_000, RecordNotAfter: 3_000_000, Continuity: 1}
	signedParent, err := record.SignRecord(network, parent, private)
	if err != nil {
		t.Fatal(err)
	}
	signedChild, err := record.SignRecord(network, child, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CommitLegacy(testEpoch(14), [][]byte{signedParent, signedChild}, thresholdAttester(signers[:2])); err != nil {
		t.Fatal(err)
	}
	proof, err := store.Lookup("site.root", 14)
	if err != nil {
		t.Fatal(err)
	}
	binding, warning, number, err := epoch.VerifyBinding(policy, proof, 14, [32]byte{14}, 1_001_000)
	if err != nil || number != 14 || binding.Name != "site.root" ||
		warning != "name lineage is in grace and should be treated as volatile" {
		t.Fatalf("parent grace binding=%+v warning=%q epoch=%d err=%v", binding, warning, number, err)
	}
}

func TestDeepestLegalNameHasCompactCurrentNamespaceProof(t *testing.T) {
	t.Parallel()
	network := [32]byte{8}
	policy, signers := materializationPolicy("deep-namespace", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	authoritySeed := sha256.Sum256([]byte("deep-name-authority"))
	authority := ed25519.NewKeyFromSeed(authoritySeed[:])
	records := make([][]byte, 127)
	for depth := 1; depth <= len(records); depth++ {
		name := strings.Repeat("a.", depth-1) + "a"
		current := record.Record{Name: name, Generation: 1, Revision: 1, Lease: "active",
			Consistency: "current", Recovery: "stable",
			Authority:      hex.EncodeToString(authority.Public().(ed25519.PublicKey)),
			LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000}
		if depth > 1 {
			current.ParentName = strings.Repeat("a.", depth-2) + "a"
			current.ParentGeneration = 1
		}
		if depth == len(records) {
			current.Target, current.RecordNotAfter = [32]byte{1}, 950_000
		}
		records[depth-1], err = record.SignRecord(network, current, authority)
		if err != nil {
			t.Fatal(err)
		}
	}
	currentEpoch := epoch.Epoch{Number: 12, Digest: [32]byte{12}, CutoffOffset: 600,
		TransitionRoot: sha256.Sum256([]byte("deep-transitions")), TransitionLength: uint32(len(records)),
		RejectionRoot: sha256.Sum256([]byte("deep-rejections"))}
	if err := store.CommitLegacy(currentEpoch, records, thresholdAttester(signers[:2])); err != nil {
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
	binding, _, number, err := epoch.VerifyBinding(policy, proof, 12, currentEpoch.Digest, 900_000)
	if err != nil || number != 12 || binding.Name != name || binding.Target != [32]byte{1} {
		t.Fatalf("binding=%+v epoch=%d err=%v", binding, number, err)
	}
}

func TestNamespaceTracerRejectsCorpusBeyondAcceptedEnvelope(t *testing.T) {
	t.Parallel()
	network := [32]byte{9}
	policy, signers := materializationPolicy("tracer-envelope", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	records := make([][]byte, 128)
	for index := range records {
		records[index] = signedRecord(t, network, fmt.Sprintf("tracer-%d", index), "tracer-authority")
	}
	currentEpoch := epoch.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("tracer-transitions")), TransitionLength: uint32(len(records)),
		RejectionRoot: sha256.Sum256([]byte("tracer-rejections"))}
	if err := store.CommitLegacy(currentEpoch, records, thresholdAttester(signers[:2])); err == nil {
		t.Fatal("corpus above the accepted 127-record tracer envelope was installed")
	}
}

func materializationPolicy(label string, network [32]byte) (epoch.MaterializationPolicy, []ed25519.PrivateKey) {
	policy := epoch.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1",
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
