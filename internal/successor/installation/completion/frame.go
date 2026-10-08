package completion

import "encoding/hex"

// Encode returns the exact closed local completion frame. The invocation must
// be nonzero and both digests must be 64 lowercase hexadecimal ASCII bytes.
// The frame carries detached identity facts, never Installation or runtime
// authority; its caller separately admits the selected generation and process.
func Encode(invocation [16]byte, generationDigest, bindingDigest string) ([160]byte, error) {
	var frame [160]byte
	if len(generationDigest) != 64 || len(bindingDigest) != 64 {
		return frame, ErrInput
	}
	hex.Encode(frame[:32], invocation[:])
	copy(frame[32:96], generationDigest)
	copy(frame[96:], bindingDigest)
	if !canonicalFrame(frame) {
		return [160]byte{}, ErrInput
	}
	return frame, nil
}

func canonicalFrame(frame [160]byte) bool {
	for _, value := range frame {
		if !(value >= '0' && value <= '9' || value >= 'a' && value <= 'f') {
			return false
		}
	}
	return string(frame[:32]) != "00000000000000000000000000000000"
}
