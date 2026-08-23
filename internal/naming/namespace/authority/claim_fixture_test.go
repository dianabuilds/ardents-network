package authority

import (
	"crypto/sha256"
	"encoding/binary"
)

func authorityClaimInputLeaf(claim Claim) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	return sha256.Sum256(append(out, claim.AdmissionDigest[:]...))
}

func authorityClaimMaterializationLeaf(claims []Claim) [32]byte {
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
