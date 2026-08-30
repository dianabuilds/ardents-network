package namespace_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
)

func TestClaimOrderVerifyUsesAuthenticatedInputOrdinal(t *testing.T) {
	t.Parallel()
	first := signedClaim(0, [32]byte{1}, claimKey("first"), "alice")
	second := signedClaim(1, [32]byte{2}, claimKey("second"), "alice")
	order, proof := authenticatedClaimSet([]claim.Claim{first, second})
	result, err := order.Verify(proof)
	if err != nil || result.Outcome != "accepted" || result.WinnerOrdinal != 0 ||
		len(result.LoserOrdinals) != 1 || result.LoserOrdinals[0] != 1 ||
		result.OperationDigest != first.Commitment {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestClaimOrderVerifyFailsClosedForHostileSets(t *testing.T) {
	base := signedClaim(0, [32]byte{7}, claimKey("claim"), "alice")
	tests := map[string]func(*claim.ClaimOrder, *claim.ClaimProof){
		"withholding": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			proof.Claims = nil
		},
		"rollback": func(order *claim.ClaimOrder, proof *claim.ClaimProof) {
			order.MinimumEpoch = proof.Epoch + 1
		},
		"copied reveal": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			copied := proof.Claims[0]
			copied.Ordinal++
			proof.Claims = append(proof.Claims, copied)
		},
		"mixed names": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			proof.Claims = append(proof.Claims, signedClaim(1, [32]byte{8}, claimKey("other"), "bob"))
		},
		"reordered reveals": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			second := signedClaim(1, [32]byte{8}, claimKey("second"), "alice")
			_, ordered := authenticatedClaimSet([]claim.Claim{proof.Claims[0], second})
			*proof = ordered
			proof.Claims[0], proof.Claims[1] = proof.Claims[1], proof.Claims[0]
		},
		"duplicate signer": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			proof.SignerIDs[1] = proof.SignerIDs[0]
		},
		"incomplete rejection evidence": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			proof.RejectionLength = 1
			signClose(proof)
		},
		"rule fork": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			proof.Rule = "ardents-name-claim-order-v2"
			signClose(proof)
		},
		"authenticated equivocation": func(_ *claim.ClaimOrder, proof *claim.ClaimProof) {
			alternate := closeOnly(*proof)
			alternate.RejectionRoot = sha256.Sum256([]byte("different rejection root"))
			alternate.RejectionLength = 1
			signClose(&alternate)
			proof.AlternateSets = []claim.ClaimProof{alternate}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			order, proof := authenticatedClaimSet([]claim.Claim{base})
			mutate(&order, &proof)
			result, err := order.Verify(proof)
			if result.Outcome == "accepted" || (result.Outcome != "fork" && err == nil) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestClaimProofWireIsBoundedCanonicalAndRoundTrips(t *testing.T) {
	t.Parallel()
	first := signedClaim(0, [32]byte{1}, claimKey("wire-first"), "alice")
	second := signedClaim(1, [32]byte{2}, claimKey("wire-second"), "alice")
	order, proof := authenticatedClaimSet([]claim.Claim{first, second})
	raw, err := claim.CanonicalProof(nil, &proof)
	if err != nil || len(raw) > 2<<10 {
		t.Fatalf("size=%d err=%v", len(raw), err)
	}
	var decoded claim.ClaimProof
	_, err = claim.CanonicalProof(raw, &decoded)
	if err != nil {
		t.Fatal(err)
	}
	result, err := order.Verify(decoded)
	if err != nil || result.Outcome != "accepted" || result.WinnerOrdinal != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := claim.CanonicalProof(append(raw, ' '), &decoded); err == nil {
		t.Fatal("non-canonical claim proof wire was accepted")
	}
}

func claimKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func signedClaim(ordinal uint32, secret [32]byte, private ed25519.PrivateKey, name string) claim.Claim {
	value := claim.Claim{Ordinal: ordinal, Name: name, Secret: secret,
		AdmissionDigest: sha256.Sum256(append([]byte("admission"), byte(ordinal)))}
	copy(value.Authority[:], private.Public().(ed25519.PublicKey))
	value.Commitment = claim.CommitmentFor([32]byte{7}, 11, value)
	copy(value.Signature[:], ed25519.Sign(private, claim.RevealTranscript([32]byte{7}, 11, value)))
	return value
}

var claimSetSigners = []ed25519.PrivateKey{claimKey("set-1"), claimKey("set-2"), claimKey("set-3")}

func authenticatedClaimSet(claims []claim.Claim) (claim.ClaimOrder, claim.ClaimProof) {
	ordered := append([]claim.Claim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Ordinal < ordered[j].Ordinal })
	leaves := make([][32]byte, len(ordered))
	for index := range ordered {
		leaves[index] = testInputLeaf(ordered[index])
	}
	for index := range ordered {
		ordered[index].InputPath = testMerklePath(leaves, index)
	}
	rejectionRoot := sha256.Sum256([]byte{2})
	if len(ordered) > 1 {
		rejections := make([][32]byte, len(ordered)-1)
		for index, claim := range ordered[1:] {
			rejections[index] = testRejectionLeaf(claim)
		}
		rejectionRoot = testMerkleRoot(rejections)
	}
	proof := claim.ClaimProof{Network: [32]byte{7}, Epoch: 11, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_000, InputRoot: testMerkleRoot(leaves), InputLength: uint32(len(leaves)),
		MaterializationRoot: testMaterializationLeaf(ordered), MaterializationLength: 1,
		RejectionRoot: rejectionRoot, RejectionLength: uint32(len(ordered) - 1), Claims: ordered}
	order := claim.ClaimOrder{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch,
		MaximumClaims: 32, Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	for _, private := range claimSetSigners {
		public := private.Public().(ed25519.PublicKey)
		order.Authorities[sha256.Sum256(public)] = public
	}
	signClose(&proof)
	return order, proof
}

func testRejectionLeaf(value claim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{3}, value.Ordinal)
	out = append(out, value.Commitment[:]...)
	out = binary.BigEndian.AppendUint32(out, uint32(len("ordered-collision")))
	out = append(out, "ordered-collision"...)
	return sha256.Sum256(out)
}

func signClose(proof *claim.ClaimProof) {
	proof.SignerIDs, proof.Signatures = nil, nil
	type signed struct {
		id  [32]byte
		raw []byte
	}
	var values []signed
	for _, private := range claimSetSigners[:2] {
		id := sha256.Sum256(private.Public().(ed25519.PublicKey))
		values = append(values, signed{id: id, raw: ed25519.Sign(private, claim.StatementTranscript(*proof))})
	}
	sort.Slice(values, func(i, j int) bool { return bytes.Compare(values[i].id[:], values[j].id[:]) < 0 })
	for _, value := range values {
		proof.SignerIDs = append(proof.SignerIDs, value.id)
		proof.Signatures = append(proof.Signatures, value.raw)
	}
}

func closeOnly(proof claim.ClaimProof) claim.ClaimProof {
	proof.Claims, proof.MaterializationPath, proof.AlternateSets = nil, nil, nil
	proof.SignerIDs, proof.Signatures = nil, nil
	return proof
}

func testInputLeaf(value claim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, value.Ordinal)
	out = append(out, value.Commitment[:]...)
	return sha256.Sum256(append(out, value.AdmissionDigest[:]...))
}

func testMaterializationLeaf(claims []claim.Claim) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	for _, claim := range claims {
		out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
		out = binary.BigEndian.AppendUint32(out, uint32(len(claim.Name)))
		out = append(out, claim.Name...)
		out = append(out, claim.Commitment[:]...)
		out = append(out, claim.Authority[:]...)
		out = append(out, claim.AdmissionDigest[:]...)
	}
	return sha256.Sum256(out)
}

func testMerkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	left, right := testMerkleRoot(leaves[:split]), testMerkleRoot(leaves[split:])
	return sha256.Sum256(append(append([]byte{1}, left[:]...), right[:]...))
}

func testMerklePath(leaves [][32]byte, index int) [][32]byte {
	if len(leaves) == 1 {
		return nil
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	if index < split {
		return append(testMerklePath(leaves[:split], index), testMerkleRoot(leaves[split:]))
	}
	return append(testMerklePath(leaves[split:], index-split), testMerkleRoot(leaves[:split]))
}
