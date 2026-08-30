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

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

type claimSigningPort func(record.RecordSigningRequest) ([]byte, error)

func (sign claimSigningPort) Sign(request record.RecordSigningRequest) ([]byte, error) {
	return sign(request)
}

func TestClaimWinnerMaterializesOnlyTheThresholdAuthenticatedWinner(t *testing.T) {
	t.Parallel()
	claimKey := deterministicAuthority("ordered-claim")
	value := claim.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{1},
		AdmissionDigest: sha256.Sum256([]byte("accepted root-claim admission"))}
	copy(value.Authority[:], claimKey.Public().(ed25519.PublicKey))
	value.Commitment = claim.CommitmentFor([32]byte{7}, 11, value)
	copy(value.Signature[:], ed25519.Sign(claimKey, claim.RevealTranscript([32]byte{7}, 11, value)))
	inputRoot := orderedInputLeaf(value)
	proof := claim.ClaimProof{Network: [32]byte{7}, Epoch: 11, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_000, InputRoot: inputRoot, InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(value), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{value}}
	order := signedClaimClose(&proof)
	winner, err := claim.OpenClaimWinner(order, proof)
	if err != nil {
		t.Fatal(err)
	}
	current, err := winner.Materialize(nil, time.Unix(100, 0).UTC(),
		record.Policy{DefaultLeaseDuration: time.Hour, DefaultGraceDuration: 30 * time.Minute})
	if err != nil || current.Name != value.Name || current.Authority != hex.EncodeToString(value.Authority[:]) ||
		current.LeaseExpiresAt != 3_700 || current.GraceExpiresAt != 5_500 {
		t.Fatalf("record=%+v err=%v", current, err)
	}
	if _, err := winner.Materialize(nil, time.Unix(101, 0).UTC(), record.Policy{}); err == nil {
		t.Fatal("one verified winner materialized more than once")
	}
}

func TestClaimWinnerDerivesReclaimInsteadOfAcceptingCallerOperation(t *testing.T) {
	t.Parallel()
	claimKey := deterministicAuthority("reclaim-winner")
	value := claim.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{2},
		AdmissionDigest: sha256.Sum256([]byte("reclaim root-claim admission"))}
	copy(value.Authority[:], claimKey.Public().(ed25519.PublicKey))
	value.Commitment = claim.CommitmentFor([32]byte{8}, 12, value)
	copy(value.Signature[:], ed25519.Sign(claimKey, claim.RevealTranscript([32]byte{8}, 12, value)))
	proof := claim.ClaimProof{Network: [32]byte{8}, Epoch: 12, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_001, InputRoot: orderedInputLeaf(value), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(value), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{value}}
	order := signedClaimClose(&proof)
	winner, err := claim.OpenClaimWinner(order, proof)
	if err != nil {
		t.Fatal(err)
	}
	previousKey := deterministicAuthority("reclaim-previous")
	previous, err := record.ApplyLegacy(nil, 90, record.Op{Kind: "claim", Name: "alice", Generation: 1,
		Authority: hex.EncodeToString(previousKey.Public().(ed25519.PublicKey))}, record.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	released, err := record.ApplyLegacy(&previous, 91, record.Op{Kind: "release", Name: "alice",
		Authority: previous.Authority, ExpectedGeneration: previous.Generation, ExpectedRevision: previous.Revision}, record.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := winner.Materialize(&released, time.Unix(100, 0).UTC(), record.Policy{})
	if err != nil || reclaimed.Generation != 2 || reclaimed.Authority != hex.EncodeToString(value.Authority[:]) {
		t.Fatalf("reclaimed=%+v err=%v", reclaimed, err)
	}
}

func TestEpochInstallationAcceptsOnlyTheDerivedSignedClaimWinner(t *testing.T) {
	t.Parallel()
	network := [32]byte{10}
	policy, attesters := materializationPolicy("epoch-claim-installation", network)
	store, err := epoch.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	claimKey := deterministicAuthority("epoch-installation-winner")
	value := claim.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{3},
		AdmissionDigest: sha256.Sum256([]byte("epoch installation claim admission"))}
	copy(value.Authority[:], claimKey.Public().(ed25519.PublicKey))
	value.Commitment = claim.CommitmentFor(network, 13, value)
	copy(value.Signature[:], ed25519.Sign(claimKey, claim.RevealTranscript(network, 13, value)))
	proof := claim.ClaimProof{Network: network, Epoch: 13, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_002, InputRoot: orderedInputLeaf(value), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(value), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{value}}
	winner, err := claim.OpenClaimWinner(signedClaimClose(&proof), proof)
	if err != nil {
		t.Fatal(err)
	}
	currentEpoch := epoch.Epoch{Number: 13, Digest: winner.CloseDigest(), CutoffOffset: 10_002,
		TransitionRoot: sha256.Sum256([]byte("claim transitions")), TransitionLength: 1,
		RejectionRoot: sha256.Sum256([]byte("claim rejections")), RejectionLength: 0}
	installation, err := store.BeginEpochInstallation(currentEpoch, time.Unix(100, 0).UTC(),
		record.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	unboundEpoch := currentEpoch
	unboundEpoch.Digest = [32]byte{13}
	unboundInstallation, err := store.BeginEpochInstallation(unboundEpoch, time.Unix(100, 0).UTC(), record.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := unboundInstallation.MaterializeClaim(winner, claimSigningPort(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(claimKey, request.Transcript()), nil
	})); err == nil {
		t.Fatal("winner was materialized into an Epoch that did not bind its close")
	}
	otherEpoch := currentEpoch
	otherEpoch.Number++
	wrongInstallation, err := store.BeginEpochInstallation(otherEpoch, time.Unix(100, 0).UTC(),
		record.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := wrongInstallation.MaterializeClaim(winner, claimSigningPort(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(claimKey, request.Transcript()), nil
	})); err == nil {
		t.Fatal("winner from a different Epoch was installed")
	}
	foreignNetwork := [32]byte{11}
	foreignClaim := value
	foreignClaim.Commitment = claim.CommitmentFor(foreignNetwork, 13, foreignClaim)
	copy(foreignClaim.Signature[:], ed25519.Sign(claimKey, claim.RevealTranscript(foreignNetwork, 13, foreignClaim)))
	foreignProof := claim.ClaimProof{Network: foreignNetwork, Epoch: 13, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_002, InputRoot: orderedInputLeaf(foreignClaim), InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(foreignClaim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []claim.Claim{foreignClaim}}
	foreignWinner, err := claim.OpenClaimWinner(signedClaimClose(&foreignProof), foreignProof)
	if err != nil {
		t.Fatal(err)
	}
	if err := installation.MaterializeClaim(foreignWinner, claimSigningPort(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(claimKey, request.Transcript()), nil
	})); err == nil {
		t.Fatal("winner from a different Network was installed")
	}
	if err := installation.MaterializeClaim(winner, claimSigningPort(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(claimKey, append(request.Transcript(), 0)), nil
	})); err == nil {
		t.Fatal("substituted signed claim Record was installed")
	}
	if err := installation.MaterializeClaim(winner, claimSigningPort(func(request record.RecordSigningRequest) ([]byte, error) {
		return ed25519.Sign(claimKey, request.Transcript()), nil
	})); err != nil {
		t.Fatalf("exact derived signed claim was denied after substitution: %v", err)
	}
	if err := installation.Commit(thresholdAttester(attesters[:2])); err != nil {
		t.Fatal(err)
	}
	current, err := store.Lookup("alice", 13)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := epoch.VerifyLegacy(policy, current, 13, currentEpoch.Digest, 100_000); err == nil || err.Error() != "name is unavailable" {
		t.Fatalf("unpublished root claim verification err=%v", err)
	}
}

func signedClaimClose(proof *claim.ClaimProof) claim.ClaimOrder {
	order := claim.ClaimOrder{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch,
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
			raw: ed25519.Sign(private, claim.StatementTranscript(*proof))})
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

func orderedInputLeaf(value claim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, value.Ordinal)
	out = append(out, value.Commitment[:]...)
	return sha256.Sum256(append(out, value.AdmissionDigest[:]...))
}

func orderedMaterializationLeaf(value claim.Claim) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	out = binary.BigEndian.AppendUint32(out, value.Ordinal)
	out = binary.BigEndian.AppendUint32(out, uint32(len(value.Name)))
	out = append(out, value.Name...)
	out = append(out, value.Commitment[:]...)
	out = append(out, value.Authority[:]...)
	out = append(out, value.AdmissionDigest[:]...)
	return sha256.Sum256(out)
}
