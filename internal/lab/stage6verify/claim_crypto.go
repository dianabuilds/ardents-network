package stage6verify

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
)

func validClaimReveal(proof claimEvidence, claim claimReveal) bool {
	if !canonicalName(claim.Name) || claim.Authority == [32]byte{} || claim.Secret == [32]byte{} ||
		claim.AdmissionDigest == [32]byte{} || claim.Ordinal >= proof.InputLength || len(claim.InputPath) > 32 {
		return false
	}
	commitment := claimCommitment(proof.Network, proof.Epoch, claim)
	return claim.Commitment == commitment && ed25519.Verify(ed25519.PublicKey(claim.Authority[:]),
		claimRevealTranscript(proof.Network, proof.Epoch, claim), claim.Signature[:]) &&
		claimInclusion(claimInputLeaf(claim), claim.Ordinal, proof.InputLength, claim.InputPath, proof.InputRoot)
}

func claimCommitment(network [32]byte, epoch uint64, claim claimReveal) [32]byte {
	out := claimText(nil, "ardents-name-claim-commit-v1")
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, epoch)
	out = claimText(out, claim.Name)
	out = append(out, claim.Authority[:]...)
	return sha256.Sum256(append(out, claim.Secret[:]...))
}

func claimRevealTranscript(network [32]byte, epoch uint64, claim claimReveal) []byte {
	out := claimText(nil, "ardents-name-claim-reveal-v1")
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, epoch)
	out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
	return append(out, claim.Commitment[:]...)
}

func claimStatement(proof claimEvidence) []byte {
	out := claimText(nil, "ardents-name-claim-epoch-close-v1")
	out = append(out, proof.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, proof.Epoch)
	out = claimText(out, proof.Rule)
	out = binary.BigEndian.AppendUint64(out, uint64(proof.CutoffOffset))
	out = append(out, proof.InputRoot[:]...)
	out = binary.BigEndian.AppendUint32(out, proof.InputLength)
	out = append(out, proof.MaterializationRoot[:]...)
	out = binary.BigEndian.AppendUint32(out, proof.MaterializationLength)
	out = append(out, proof.RejectionRoot[:]...)
	return binary.BigEndian.AppendUint32(out, proof.RejectionLength)
}

func claimInputLeaf(claim claimReveal) [32]byte {
	out := binary.BigEndian.AppendUint32([]byte{0}, claim.Ordinal)
	out = append(out, claim.Commitment[:]...)
	return sha256.Sum256(append(out, claim.AdmissionDigest[:]...))
}

func claimMaterializationLeaf(claims []claimReveal) [32]byte {
	out := []byte("ardents-name-claim-materialization-v1\x00")
	for _, claim := range claims {
		out = binary.BigEndian.AppendUint32(out, claim.Ordinal)
		out = claimText(out, claim.Name)
		out = append(out, claim.Commitment[:]...)
		out = append(out, claim.Authority[:]...)
		out = append(out, claim.AdmissionDigest[:]...)
	}
	return sha256.Sum256(out)
}

func claimInclusion(leaf [32]byte, index, length uint32, path [][32]byte, root [32]byte) bool {
	if length == 0 || index >= length {
		return false
	}
	value, fn, sn := leaf, index, length-1
	for _, sibling := range path {
		if fn&1 == 1 || fn == sn {
			value = claimNode(sibling, value)
			for fn&1 == 0 && fn != 0 {
				fn, sn = fn>>1, sn>>1
			}
		} else {
			value = claimNode(value, sibling)
		}
		fn, sn = fn>>1, sn>>1
	}
	return sn == 0 && value == root
}

func claimNode(left, right [32]byte) [32]byte {
	out := append([]byte{1}, left[:]...)
	return sha256.Sum256(append(out, right[:]...))
}

func claimText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
