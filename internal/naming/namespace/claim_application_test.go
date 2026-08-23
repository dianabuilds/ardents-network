package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

func TestClaimWinnerMaterializesOnlyTheThresholdAuthenticatedWinner(t *testing.T) {
	t.Parallel()
	claimKey := deterministicAuthority("ordered-claim")
	claim := namespace.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{1},
		AdmissionDigest: sha256.Sum256([]byte("accepted root-claim admission"))}
	copy(claim.Authority[:], claimKey.Public().(ed25519.PublicKey))
	claim.Commitment = namespace.CommitmentFor([32]byte{7}, 11, claim)
	copy(claim.Signature[:], ed25519.Sign(claimKey, namespace.RevealTranscript([32]byte{7}, 11, claim)))
	inputRoot := orderedInputLeaf(claim)
	proof := namespace.ClaimProof{Network: [32]byte{7}, Epoch: 11, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_000, InputRoot: inputRoot, InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(claim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{claim}}
	order := signedClaimClose(&proof)
	winner, err := namespace.OpenClaimWinner(order, proof)
	if err != nil {
		t.Fatal(err)
	}
	record, err := winner.Materialize(nil, time.Unix(100, 0).UTC(),
		namespace.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil || record.Name != claim.Name || record.Authority != hex.EncodeToString(claim.Authority[:]) {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	if _, err := winner.Materialize(nil, time.Unix(101, 0).UTC(), namespace.Policy{}); err == nil {
		t.Fatal("one verified winner materialized more than once")
	}
}

func TestClaimWinnerDerivesReclaimInsteadOfAcceptingCallerOperation(t *testing.T) {
	t.Parallel()
	claimKey := deterministicAuthority("reclaim-winner")
	claim := namespace.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{2},
		AdmissionDigest: sha256.Sum256([]byte("reclaim root-claim admission"))}
	copy(claim.Authority[:], claimKey.Public().(ed25519.PublicKey))
	claim.Commitment = namespace.CommitmentFor([32]byte{8}, 12, claim)
	copy(claim.Signature[:], ed25519.Sign(claimKey, namespace.RevealTranscript([32]byte{8}, 12, claim)))
	proof := namespace.ClaimProof{Network: [32]byte{8}, Epoch: 12, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_001, InputRoot: orderedInputLeaf(claim), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(claim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{claim}}
	order := signedClaimClose(&proof)
	winner, err := namespace.OpenClaimWinner(order, proof)
	if err != nil {
		t.Fatal(err)
	}
	previousKey := deterministicAuthority("reclaim-previous")
	previous, err := namespace.ApplyLegacy(nil, 90, namespace.Op{Kind: "claim", Name: "alice", Generation: 1,
		Authority: hex.EncodeToString(previousKey.Public().(ed25519.PublicKey))}, namespace.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	released, err := namespace.ApplyLegacy(&previous, 91, namespace.Op{Kind: "release", Name: "alice",
		Authority: previous.Authority, ExpectedGeneration: previous.Generation, ExpectedRevision: previous.Revision}, namespace.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := winner.Materialize(&released, time.Unix(100, 0).UTC(), namespace.Policy{})
	if err != nil || reclaimed.Generation != 2 || reclaimed.Authority != hex.EncodeToString(claim.Authority[:]) {
		t.Fatalf("reclaimed=%+v err=%v", reclaimed, err)
	}
}

func TestEpochInstallationAcceptsOnlyTheDerivedSignedClaimWinner(t *testing.T) {
	t.Parallel()
	network := [32]byte{10}
	policy, attesters := materializationPolicy("epoch-claim-installation", network)
	store, err := namespace.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	claimKey := deterministicAuthority("epoch-installation-winner")
	claim := namespace.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{3},
		AdmissionDigest: sha256.Sum256([]byte("epoch installation claim admission"))}
	copy(claim.Authority[:], claimKey.Public().(ed25519.PublicKey))
	claim.Commitment = namespace.CommitmentFor(network, 13, claim)
	copy(claim.Signature[:], ed25519.Sign(claimKey, namespace.RevealTranscript(network, 13, claim)))
	proof := namespace.ClaimProof{Network: network, Epoch: 13, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_002, InputRoot: orderedInputLeaf(claim), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(claim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{claim}}
	winner, err := namespace.OpenClaimWinner(signedClaimClose(&proof), proof)
	if err != nil {
		t.Fatal(err)
	}
	epoch := namespace.Epoch{Number: 13, Digest: [32]byte{13}, CutoffOffset: 10_002,
		TransitionRoot: sha256.Sum256([]byte("claim transitions")), TransitionLength: 1,
		RejectionRoot: sha256.Sum256([]byte("claim rejections")), RejectionLength: 0}
	installation, err := store.BeginEpochInstallation(epoch, time.Unix(100, 0).UTC(),
		namespace.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	otherEpoch := epoch
	otherEpoch.Number++
	wrongInstallation, err := store.BeginEpochInstallation(otherEpoch, time.Unix(100, 0).UTC(),
		namespace.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongInstallation.MaterializeClaim(winner, func(record namespace.Record) ([]byte, error) {
		return namespace.SignRecord(network, record, claimKey)
	}); err == nil {
		t.Fatal("winner from a different Epoch was installed")
	}
	foreignNetwork := [32]byte{11}
	foreignClaim := claim
	foreignClaim.Commitment = namespace.CommitmentFor(foreignNetwork, 13, foreignClaim)
	copy(foreignClaim.Signature[:], ed25519.Sign(claimKey, namespace.RevealTranscript(foreignNetwork, 13, foreignClaim)))
	foreignProof := namespace.ClaimProof{Network: foreignNetwork, Epoch: 13, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_002, InputRoot: orderedInputLeaf(foreignClaim), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(foreignClaim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []namespace.Claim{foreignClaim}}
	foreignWinner, err := namespace.OpenClaimWinner(signedClaimClose(&foreignProof), foreignProof)
	if err != nil {
		t.Fatal(err)
	}
	if err := installation.MaterializeClaim(foreignWinner, func(record namespace.Record) ([]byte, error) {
		return namespace.SignRecord(network, record, claimKey)
	}); err == nil {
		t.Fatal("winner from a different Network was installed")
	}
	if err := installation.MaterializeClaim(winner, func(record namespace.Record) ([]byte, error) {
		record.Continuity++
		return namespace.SignRecord(network, record, claimKey)
	}); err == nil {
		t.Fatal("substituted signed claim Record was installed")
	}
	if err := installation.MaterializeClaim(winner, func(record namespace.Record) ([]byte, error) {
		return namespace.SignRecord(network, record, claimKey)
	}); err != nil {
		t.Fatalf("exact derived signed claim was denied after substitution: %v", err)
	}
	if err := installation.Commit(thresholdAttester(attesters[:2])); err != nil {
		t.Fatal(err)
	}
	current, err := store.Lookup("alice", 13)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := namespace.Verify(policy, current, 13, epoch.Digest, 100_000); err == nil || err.Error() != "name is unavailable" {
		t.Fatalf("unpublished root claim verification err=%v", err)
	}
}

func signedClaimClose(proof *namespace.ClaimProof) namespace.ClaimOrder {
	order := namespace.ClaimOrder{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch,
		MaximumClaims: 32, Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	type signed struct {
		id  [32]byte
		raw []byte
	}
	var signatures []signed
	for _, label := range []string{"epoch-1", "epoch-2"} {
		private := deterministicAuthority(label)
		public := private.Public().(ed25519.PublicKey)
		id := sha256.Sum256(public)
		order.Authorities[id] = public
		signatures = append(signatures, signed{id: id,
			raw: ed25519.Sign(private, namespace.StatementTranscript(*proof))})
	}
	sort.Slice(signatures, func(i, j int) bool {
		return bytes.Compare(signatures[i].id[:], signatures[j].id[:]) < 0
	})
	for _, signature := range signatures {
		proof.SignerIDs = append(proof.SignerIDs, signature.id)
		proof.Signatures = append(proof.Signatures, signature.raw)
	}
	return order
}

func orderedInputLeaf(claim namespace.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	return sha256.Sum256(append(out, claim.AdmissionDigest[:]...))
}

func orderedMaterializationLeaf(claim namespace.Claim) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
	out = binary.BigEndian.AppendUint32(out, uint32(len(claim.Name)))
	out = append(out, claim.Name...)
	out = append(out, claim.Commitment[:]...)
	out = append(out, claim.Authority[:]...)
	out = append(out, claim.AdmissionDigest[:]...)
	return sha256.Sum256(out)
}
