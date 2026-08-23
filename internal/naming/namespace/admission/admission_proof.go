package admission

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
)

// BindsIsolation checks the local raw Isolation Context against the opaque,
// per-challenge binding visible to the admitting Node.
func (challenge Challenge) BindsIsolation(isolation [32]byte) bool {
	want := isolationBinding(challenge.Node, challenge.OperationDigest, isolation, challenge.Nonce)
	return subtle.ConstantTimeCompare(want[:], challenge.IsolationBinding[:]) == 1
}

// Solve performs the bounded client-side search for this Challenge.
func (challenge Challenge) Solve() (Proof, uint64) {
	input := append(challengeBytes(challenge, true), make([]byte, 8)...)
	for nonce := uint64(0); ; nonce++ {
		binary.BigEndian.PutUint64(input[len(input)-8:], nonce)
		if validWorkDigest(sha256.Sum256(input), challenge.WorkBits) {
			return Proof{Challenge: challenge, WorkNonce: nonce}, nonce + 1
		}
	}
}

func validWork(challenge Challenge, nonce uint64) bool {
	input := challengeBytes(challenge, true)
	input = binary.BigEndian.AppendUint64(input, nonce)
	return validWorkDigest(sha256.Sum256(input), challenge.WorkBits)
}

func validWorkDigest(digest [32]byte, workBits uint8) bool {
	bits := int(workBits)
	for bits >= 8 {
		if digest[(int(workBits)-bits)/8] != 0 {
			return false
		}
		bits -= 8
	}
	return bits == 0 || digest[int(workBits)/8]>>(8-bits) == 0
}

func challengeDigest(challenge Challenge) [32]byte {
	return sha256.Sum256(challengeBytes(challenge, true))
}

func challengeBytes(challenge Challenge, includeTag bool) []byte {
	out := admissionText(nil, "ardents-name-admission-challenge-v1")
	out = append(out, challenge.Node[:]...)
	out = append(out, challenge.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, challenge.Epoch)
	out = admissionText(out, challenge.Surface)
	out = append(out, challenge.OperationDigest[:]...)
	out = append(out, challenge.IsolationBinding[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.IssuedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.ExpiresAt))
	out = append(out, challenge.Nonce[:]...)
	out = append(out, challenge.WorkBits)
	if includeTag {
		out = append(out, challenge.AuthenticationTag[:]...)
	}
	return out
}

func admissionText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}

func isolationBinding(node, operation, isolation [32]byte, nonce [16]byte) [32]byte {
	out := []byte("ardents-name-admission-isolation-v1\x00")
	out = append(out, node[:]...)
	out = append(out, operation[:]...)
	out = append(out, isolation[:]...)
	out = append(out, nonce[:]...)
	return sha256.Sum256(out)
}
