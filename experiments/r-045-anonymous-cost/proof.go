//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/binary"
)

func validWork(challenge Challenge, nonce uint64) bool {
	input := challengeBytes(challenge, true)
	input = binary.BigEndian.AppendUint64(input, nonce)
	digest := sha256.Sum256(input)
	bits := int(challenge.WorkBits)
	for bits >= 8 {
		if digest[(int(challenge.WorkBits)-bits)/8] != 0 {
			return false
		}
		bits -= 8
	}
	if bits == 0 {
		return true
	}
	return digest[int(challenge.WorkBits)/8]>>(8-bits) == 0
}

func challengeBytes(challenge Challenge, includeTag bool) []byte {
	out := appendText(nil, "ardents-name-admission-challenge-v1")
	out = append(out, challenge.Node[:]...)
	out = append(out, challenge.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, challenge.Epoch)
	out = appendText(out, string(challenge.Surface))
	out = append(out, challenge.OperationDigest[:]...)
	out = append(out, challenge.IsolationContext[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.IssuedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.ExpiresAt))
	out = append(out, challenge.Nonce[:]...)
	out = append(out, challenge.WorkBits)
	if includeTag {
		out = append(out, challenge.AuthenticationTag[:]...)
	}
	return out
}

func appendText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
