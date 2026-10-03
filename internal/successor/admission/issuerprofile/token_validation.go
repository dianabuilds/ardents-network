package issuerprofile

// ValidateTokenSPKI shares the canonical key parser used by profile preparation
// and verification. A noncanonical equivalent key is not the same token key.
func ValidateTokenSPKI(raw []byte) bool {
	_, valid := ParseKey(raw)
	return valid
}
