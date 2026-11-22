package main

import "encoding/hex"

// decodeHex32 decodes exactly 32 bytes from canonical lowercase hex. It reports
// whether the input is well-formed and returns the zero value otherwise; each
// caller owns its error classification and zero-value policy.
func decodeHex32(encoded string) ([32]byte, bool) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) || hex.EncodeToString(raw) != encoded {
		return value, false
	}
	copy(value[:], raw)
	return value, true
}
