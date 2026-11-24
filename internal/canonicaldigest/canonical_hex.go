package canonicaldigest

import "encoding/hex"

// Decode decodes exactly 32 bytes from canonical lowercase hex. It reports
// whether the input is well-formed and returns the zero value otherwise; each
// caller owns its error classification and zero-value policy.
func Decode(value string) ([32]byte, bool) {
	var digest [32]byte
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != len(digest) || hex.EncodeToString(raw) != value {
		return digest, false
	}
	copy(digest[:], raw)
	return digest, true
}

// Valid reports whether value is canonical lowercase hex of exactly 32 bytes.
// It is the validation-only projection of Decode and admits the same inputs.
func Valid(value string) bool {
	_, ok := Decode(value)
	return ok
}
