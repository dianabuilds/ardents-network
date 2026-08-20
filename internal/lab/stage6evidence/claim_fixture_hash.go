package stage6evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/nameclaim"
)

func evidenceInputLeaf(claim nameclaim.Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	return sha256.Sum256(append(out, claim.AdmissionDigest[:]...))
}

func evidenceMaterializationLeaf(claims []nameclaim.Claim) [32]byte {
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

func evidenceMerkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	left, right := evidenceMerkleRoot(leaves[:split]), evidenceMerkleRoot(leaves[split:])
	return sha256.Sum256(append(append([]byte{1}, left[:]...), right[:]...))
}

func evidenceMerklePath(leaves [][32]byte, index int) [][32]byte {
	if len(leaves) == 1 {
		return nil
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	if index < split {
		return append(evidenceMerklePath(leaves[:split], index), evidenceMerkleRoot(leaves[split:]))
	}
	return append(evidenceMerklePath(leaves[split:], index-split), evidenceMerkleRoot(leaves[:split]))
}

func cloneClaimProof(proof nameclaim.Proof) nameclaim.Proof {
	raw, _ := json.Marshal(proof)
	var copied nameclaim.Proof
	_ = json.Unmarshal(raw, &copied)
	return copied
}

func closeClaimProof(proof nameclaim.Proof) nameclaim.Proof {
	proof.Claims, proof.MaterializationPath, proof.AlternateSets = nil, nil, nil
	proof.SignerIDs, proof.Signatures = nil, nil
	return proof
}

func sortClaimAuthorities(raw []byte) {
	if len(raw) == 128 && bytes.Compare(raw[:64], raw[64:]) > 0 {
		copyRaw := append([]byte(nil), raw[:64]...)
		copy(raw[:64], raw[64:])
		copy(raw[64:], copyRaw)
	}
}
