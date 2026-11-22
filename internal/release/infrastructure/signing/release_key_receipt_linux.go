package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
)

// writeReleaseKeyReceipt reports the five public keys after the selected
// preparation command has finished writing its private files. All four roles
// refuse a wrong argument count with the bare shared input error; preparation
// failures keep their own role failure classification (audit E1-6, #718).
func writeReleaseKeyReceipt(ctx context.Context, out io.Writer, operation string, public [5][32]byte) error {
	type publicKey struct {
		File   string `json:"file"`
		Public string `json:"ed25519_public_hex"`
		Digest string `json:"public_sha256"`
	}
	keys := make([]publicKey, 0, len(public))
	for i, pub := range public {
		digest := sha256.Sum256(pub[:])
		keys = append(keys, publicKey{fmt.Sprintf("release-key-%d.pem", i+1), hex.EncodeToString(pub[:]), hex.EncodeToString(digest[:])})
	}
	raw, err := json.Marshal(struct {
		Operation   string      `json:"operation"`
		Threshold   int         `json:"ordinary_threshold"`
		Independent bool        `json:"independent_custody"`
		Keys        []publicKey `json:"keys"`
	}{operation, 3, false, keys})
	if err != nil {
		return err
	}
	return artifactio.WriteReceipt(ctx, out, append(raw, '\n'))
}
