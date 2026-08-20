package stage6evidence

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/nameclaim"
)

var evidenceClaimSigners = []ed25519.PrivateKey{evidenceKey("claim-set-1"), evidenceKey("claim-set-2")}

type claimTraceEvidence struct {
	Primary    nameclaim.Proof
	Inputs     []claimInputEvidence
	Rejections []claimRejectionEvidence
	Hostile    []nameclaim.Proof
}

type claimInputEvidence struct {
	Ordinal         uint32
	Commitment      [32]byte
	AdmissionDigest [32]byte
}

type claimRejectionEvidence struct {
	Ordinal    uint32
	Commitment [32]byte
	Reason     string
}

func runClaimCell(trace *traceRecord) error {
	first := evidenceClaim(0, [32]byte{1}, evidenceKey("claim-first"))
	second := evidenceClaim(1, [32]byte{2}, evidenceKey("claim-second"))
	order, proof := evidenceClaimSet([]nameclaim.Claim{first, second})
	switch trace.Cell {
	case "C4":
		result, err := order.Verify(proof)
		if err != nil || result.Outcome != "accepted" || result.WinnerOrdinal != 0 ||
			len(result.LoserOrdinals) != 1 || result.LoserOrdinals[0] != 1 {
			return errors.New("authenticated claim order did not select one ordinal winner")
		}
		trace.Fields = []string{"accepted", "ordered-collision"}
	case "C5":
		proof.Claims[0].Signature[0]++
		result, err := order.Verify(proof)
		if err == nil || result.Outcome != "conflict" {
			return errors.New("unprovable claim order did not expose conflict")
		}
		trace.Fields = []string{"conflict"}
	case "C6":
		hostile := hostileClaimProofs(proof)
		outcomes := hostileClaimOutcomes(order, hostile)
		want := []string{"unavailable", "unavailable", "fork", "fork", "conflict"}
		if len(outcomes) != len(want) {
			return errors.New("claim hostile matrix is incomplete")
		}
		for index := range want {
			if outcomes[index] != want[index] {
				return errors.New("claim hostile matrix accepted a forbidden outcome")
			}
		}
		trace.Fields = outcomes
		raw, err := json.Marshal(claimTraceEvidence{Primary: proof, Inputs: claimInputs(proof),
			Rejections: claimRejections(proof), Hostile: hostile})
		if err != nil {
			return err
		}
		trace.Auxiliary = raw
	}
	if trace.Cell != "C6" {
		raw, err := json.Marshal(claimTraceEvidence{Primary: proof, Inputs: claimInputs(proof),
			Rejections: claimRejections(proof)})
		if err != nil {
			return err
		}
		trace.Auxiliary = raw
	}
	trace.Values = []int64{int64(order.MinimumEpoch), int64(order.MaximumClaims), int64(order.Threshold)}
	for id, public := range order.Authorities {
		trace.Input = append(trace.Input, id[:]...)
		trace.Input = append(trace.Input, public...)
	}
	sortClaimAuthorities(trace.Input)
	return nil
}

func claimInputs(proof nameclaim.Proof) []claimInputEvidence {
	inputs := make([]claimInputEvidence, len(proof.Claims))
	for index, claim := range proof.Claims {
		inputs[index] = claimInputEvidence{Ordinal: claim.Ordinal, Commitment: claim.Commitment,
			AdmissionDigest: claim.AdmissionDigest}
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Ordinal < inputs[j].Ordinal })
	return inputs
}

func claimRejections(proof nameclaim.Proof) []claimRejectionEvidence {
	claims := append([]nameclaim.Claim(nil), proof.Claims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].Ordinal < claims[j].Ordinal })
	rejections := make([]claimRejectionEvidence, 0, len(claims)-1)
	for _, claim := range claims[1:] {
		rejections = append(rejections, claimRejectionEvidence{Ordinal: claim.Ordinal,
			Commitment: claim.Commitment, Reason: "ordered-collision"})
	}
	return rejections
}

func hostileClaimProofs(proof nameclaim.Proof) []nameclaim.Proof {
	withheld := cloneClaimProof(proof)
	withheld.Claims = withheld.Claims[:1]
	equivocation := cloneClaimProof(proof)
	alternate := closeClaimProof(proof)
	alternate.RejectionRoot = sha256.Sum256([]byte("equivocation"))
	alternate.RejectionLength = 1
	signEvidenceClaimClose(&alternate)
	equivocation.AlternateSets = []nameclaim.Proof{alternate}
	ruleFork := cloneClaimProof(proof)
	ruleFork.Rule = "ardents-name-claim-order-v2"
	signEvidenceClaimClose(&ruleFork)
	copied := cloneClaimProof(proof)
	copyReveal := copied.Claims[0]
	copyReveal.Ordinal = 2
	copied.Claims = append(copied.Claims, copyReveal)
	return []nameclaim.Proof{withheld, proof, equivocation, ruleFork, copied}
}

func hostileClaimOutcomes(order nameclaim.ClaimOrder, proofs []nameclaim.Proof) []string {
	outcomes := make([]string, len(proofs))
	for index, proof := range proofs {
		policy := order
		if index == 1 {
			policy.MinimumEpoch++
		}
		result, _ := policy.Verify(proof)
		outcomes[index] = result.Outcome
	}
	return outcomes
}

func evidenceClaim(ordinal uint32, secret [32]byte, private ed25519.PrivateKey) nameclaim.Claim {
	return evidenceClaimFor([32]byte{7}, 11, ordinal, secret, private)
}

func evidenceClaimFor(network [32]byte, epoch uint64, ordinal uint32, secret [32]byte,
	private ed25519.PrivateKey,
) nameclaim.Claim {
	return evidenceNamedClaimFor(network, epoch, ordinal, secret, private, "alice")
}

func evidenceNamedClaimFor(network [32]byte, epoch uint64, ordinal uint32, secret [32]byte,
	private ed25519.PrivateKey, name string,
) nameclaim.Claim {
	claim := nameclaim.Claim{Ordinal: ordinal, Name: name, Secret: secret,
		AdmissionDigest: sha256.Sum256(append([]byte("root-claim-admission"), byte(ordinal)))}
	copy(claim.Authority[:], private.Public().(ed25519.PublicKey))
	claim.Commitment = nameclaim.CommitmentFor(network, epoch, claim)
	copy(claim.Signature[:], ed25519.Sign(private, nameclaim.RevealTranscript(network, epoch, claim)))
	return claim
}

func evidenceClaimSet(claims []nameclaim.Claim) (nameclaim.ClaimOrder, nameclaim.Proof) {
	return evidenceClaimSetFor([32]byte{7}, 11, claims)
}

func evidenceClaimSetFor(network [32]byte, epoch uint64,
	claims []nameclaim.Claim,
) (nameclaim.ClaimOrder, nameclaim.Proof) {
	ordered := append([]nameclaim.Claim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Ordinal < ordered[j].Ordinal })
	leaves := make([][32]byte, len(ordered))
	for index := range ordered {
		leaves[index] = evidenceInputLeaf(ordered[index])
	}
	for index := range ordered {
		ordered[index].InputPath = evidenceMerklePath(leaves, index)
	}
	rejectionRoot := sha256.Sum256([]byte{2})
	if len(ordered) > 1 {
		rejectionLeaves := make([][32]byte, len(ordered)-1)
		for index, claim := range ordered[1:] {
			rejectionLeaves[index] = evidenceRejectionLeaf(claim)
		}
		rejectionRoot = evidenceMerkleRoot(rejectionLeaves)
	}
	proof := nameclaim.Proof{Network: network, Epoch: epoch, Rule: "ardents-name-claim-order-v1",
		CutoffOffset: 10_000, InputRoot: evidenceMerkleRoot(leaves), InputLength: uint32(len(leaves)),
		MaterializationRoot: evidenceMaterializationLeaf(ordered), MaterializationLength: 1,
		RejectionRoot: rejectionRoot, RejectionLength: uint32(len(ordered) - 1), Claims: ordered}
	order := nameclaim.ClaimOrder{Network: proof.Network, Rule: proof.Rule, MinimumEpoch: proof.Epoch,
		MaximumClaims: 32, Authorities: map[[32]byte]ed25519.PublicKey{}, Threshold: 2}
	for _, signer := range evidenceClaimSigners {
		public := signer.Public().(ed25519.PublicKey)
		order.Authorities[sha256.Sum256(public)] = public
	}
	signEvidenceClaimClose(&proof)
	return order, proof
}

func evidenceRejectionLeaf(claim nameclaim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{3}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	out = binary.BigEndian.AppendUint32(out, uint32(len("ordered-collision")))
	out = append(out, "ordered-collision"...)
	return sha256.Sum256(out)
}

func signEvidenceClaimClose(proof *nameclaim.Proof) {
	proof.SignerIDs, proof.Signatures = nil, nil
	type signed struct {
		id  [32]byte
		raw []byte
	}
	var signatures []signed
	for _, private := range evidenceClaimSigners {
		id := sha256.Sum256(private.Public().(ed25519.PublicKey))
		signatures = append(signatures, signed{id: id,
			raw: ed25519.Sign(private, nameclaim.StatementTranscript(*proof))})
	}
	sort.Slice(signatures, func(i, j int) bool { return bytes.Compare(signatures[i].id[:], signatures[j].id[:]) < 0 })
	for _, signature := range signatures {
		proof.SignerIDs = append(proof.SignerIDs, signature.id)
		proof.Signatures = append(proof.Signatures, signature.raw)
	}
}
