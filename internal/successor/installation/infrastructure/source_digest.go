package infrastructure

import (
	"crypto/sha256"
	"encoding/hex"
)

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
