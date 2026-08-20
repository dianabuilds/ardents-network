package nameauthority_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/nameclaim"
	"github.com/dianabuilds/ardents-network/internal/namelease"
)

func TestOrderedClaimMaterializesOnlyTheThresholdAuthenticatedWinner(t *testing.T) {
	t.Parallel()
	claimKey := deterministicAuthority("ordered-claim")
	claim := nameclaim.Claim{Ordinal: 0, Name: "alice", Secret: [32]byte{1},
		AdmissionDigest: sha256.Sum256([]byte("accepted root-claim admission"))}
	copy(claim.Authority[:], claimKey.Public().(ed25519.PublicKey))
	claim.Commitment = nameclaim.CommitmentFor([32]byte{7}, 11, claim)
	copy(claim.Signature[:], ed25519.Sign(claimKey, nameclaim.RevealTranscript([32]byte{7}, 11, claim)))
	inputRoot := orderedInputLeaf(claim)
	proof := nameclaim.Proof{Network: [32]byte{7}, Epoch: 11, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_000, InputRoot: inputRoot, InputLength: 1,
		MaterializationRoot: orderedMaterializationLeaf(claim), MaterializationLength: 1,
		RejectionRoot: sha256.Sum256([]byte{2}), Claims: []nameclaim.Claim{claim}}
	order := signedClaimClose(&proof)
	op := namelease.Op{Kind: "claim", Name: claim.Name, Generation: 1, ClaimOrdinal: claim.Ordinal,
		Authority: hex.EncodeToString(claim.Authority[:])}
	record, err := nameauthority.ApplyOrderedClaim(order, proof, nil, 100, op,
		namelease.Policy{DefaultLeaseDuration: time.Hour})
	if err != nil || record.Name != claim.Name || record.Authority != op.Authority {
		t.Fatalf("record=%+v err=%v", record, err)
	}
	op.ClaimOrdinal++
	if _, err := nameauthority.ApplyOrderedClaim(order, proof, nil, 100, op, namelease.Policy{}); err == nil {
		t.Fatal("non-winning ordinal was materialized")
	}
}

func signedClaimClose(proof *nameclaim.Proof) nameclaim.ClaimOrder {
	order := nameclaim.ClaimOrder{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch,
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
			raw: ed25519.Sign(private, nameclaim.StatementTranscript(*proof))})
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

func orderedInputLeaf(claim nameclaim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	return sha256.Sum256(append(out, claim.AdmissionDigest[:]...))
}

func orderedMaterializationLeaf(claim nameclaim.Claim) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
	out = binary.BigEndian.AppendUint32(out, uint32(len(claim.Name)))
	out = append(out, claim.Name...)
	out = append(out, claim.Commitment[:]...)
	out = append(out, claim.Authority[:]...)
	out = append(out, claim.AdmissionDigest[:]...)
	return sha256.Sum256(out)
}
