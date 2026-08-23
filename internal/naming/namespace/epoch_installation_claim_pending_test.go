package namespace

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"
)

func TestEpochInstallationCommitsVerifiedClaimWithSelectedPendingPrefix(t *testing.T) {
	now, network := time.Unix(1_800_000_400, 0).UTC(), [32]byte{5}
	policy, attesters := pendingTestPolicy(network)
	store, err := Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	currentKey := deterministicControlKey("epoch-installation-pending-current")
	current := controlTestRecord("alice", currentKey, now)
	signedCurrent, err := SignRecord(network, current, currentKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: [32]byte{2}, TransitionLength: 1, RejectionRoot: [32]byte{3}},
		[][]byte{signedCurrent}, pendingTestAttester(attesters)); err != nil {
		t.Fatal(err)
	}
	gate, err := NewAdmission([32]byte{2}, network, 1, [32]byte{4})
	if err != nil {
		t.Fatal(err)
	}
	leasePolicy := Policy{DefaultLeaseDuration: time.Hour}
	control, err := OpenControl(store, gate, ClaimOrder{}, func() time.Time { return now }, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	updated := durableRenew(t, network, current, currentKey, now, leasePolicy)
	if class := durableSubmit(t, control, gate, network, now, updated, current, currentKey, 11); class != "submitted" {
		t.Fatalf("pending transition=%q", class)
	}
	claimKey := deterministicControlKey("epoch-installation-pending-claim")
	winner := pendingClaimWinner(t, network, 2, "bob", claimKey)
	epoch := Epoch{Number: 2, Digest: [32]byte{4}, CutoffOffset: 2,
		TransitionRoot: [32]byte{5}, TransitionLength: 1, RejectionRoot: [32]byte{6}}
	installation, err := store.BeginEpochInstallation(epoch, now, leasePolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := installation.IncludePendingThrough(1); err != nil {
		t.Fatal(err)
	}
	if err := installation.MaterializeClaim(winner, func(record Record) ([]byte, error) {
		return SignRecord(network, record, claimKey)
	}); err != nil {
		t.Fatal(err)
	}
	if err := installation.Commit(pendingTestAttester(attesters)); err != nil {
		t.Fatalf("mixed Epoch installation: %v", err)
	}
	if _, err := store.Lookup("alice", 2); err != nil {
		t.Fatalf("selected pending Record was not current: %v", err)
	}
	if _, err := store.Lookup("bob", 2); err != nil {
		t.Fatalf("verified root claim was not current: %v", err)
	}
}

func TestEpochInstallationRejectsChangedCurrentGeneration(t *testing.T) {
	now, network := time.Unix(1_800_000_500, 0).UTC(), [32]byte{4}
	policy, attesters := pendingTestPolicy(network)
	store, err := Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	epoch := Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: [32]byte{2}, TransitionLength: 1, RejectionRoot: [32]byte{3}}
	installation, err := store.BeginEpochInstallation(epoch, now, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	key := deterministicControlKey("epoch-installation-stale-current")
	record := controlTestRecord("alice", key, now)
	signed, err := SignRecord(network, record, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(epoch, [][]byte{signed}, pendingTestAttester(attesters)); err != nil {
		t.Fatal(err)
	}
	if err := installation.Commit(pendingTestAttester(attesters)); err == nil {
		t.Fatal("installation built from an older current generation committed")
	}
}

func pendingClaimWinner(t *testing.T, network [32]byte, epoch uint64, name string,
	claimKey ed25519.PrivateKey,
) *ClaimWinner {
	t.Helper()
	claim := Claim{Ordinal: 0, Name: name, Secret: [32]byte{7}, AdmissionDigest: [32]byte{8}}
	copy(claim.Authority[:], claimKey.Public().(ed25519.PublicKey))
	claim.Commitment = CommitmentFor(network, epoch, claim)
	copy(claim.Signature[:], ed25519.Sign(claimKey, RevealTranscript(network, epoch, claim)))
	proof := ClaimProof{Network: network, Epoch: epoch, Rule: claimOrderRule, CutoffOffset: 2,
		InputRoot: claimInputLeaf(claim), InputLength: 1,
		MaterializationRoot: claimMaterializationLeaf([]Claim{claim}), MaterializationLength: 1,
		RejectionRoot: emptyClaimRoot(), Claims: []Claim{claim}}
	order := ClaimOrder{Network: network, Rule: claimOrderRule, MinimumEpoch: epoch,
		MaximumClaims: 32, Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	keys := []ed25519.PrivateKey{deterministicControlKey("epoch-installation-pending-close-a"),
		deterministicControlKey("epoch-installation-pending-close-b")}
	for _, key := range keys {
		public := key.Public().(ed25519.PublicKey)
		order.Authorities[sha256.Sum256(public)] = public
	}
	proof.SignerIDs, proof.Signatures, _ = pendingTestAttester(keys)(StatementTranscript(proof))
	winner, err := OpenClaimWinner(order, proof)
	if err != nil {
		t.Fatal(err)
	}
	return winner
}
