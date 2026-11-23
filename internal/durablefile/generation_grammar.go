package durablefile

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
)

// GenerationName is the exact content-addressed generation filename grammar
// shared by every owner-only generation store: a lowercase hex SHA-256 digest.
var GenerationName = regexp.MustCompile(`^[0-9a-f]{64}$`)

// SHA256Hex returns the lowercase hex SHA-256 digest of raw, the canonical
// generation name for those bytes.
func SHA256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
