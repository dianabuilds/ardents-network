package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"

	identityprincipal "ardents/internal/identity/principal"
)

func PrincipalFromPublicKey(encoded string) (string, error) {
	return identityprincipal.FromPublicKey(encoded)
}

func deriveID(prefix string, raw []byte) string {
	sum := sha256.Sum256(raw)
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
