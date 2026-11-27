//go:build linux || darwin

package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure/signing/internal/keyset"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// writeReleaseKeyReceipt reports the five public keys after the selected
// preparation command has finished writing its private files. All four roles
// refuse a wrong argument count with the bare shared input error; preparation
// failures keep their own role failure classification (audit E1-6, #718).
func writeReleaseKeyReceipt(ctx context.Context, out io.Writer, operation string, public keyset.Inventory) error {
	type publicKey struct {
		File   string `json:"file"`
		Public string `json:"ed25519_public_hex"`
		Digest string `json:"public_sha256"`
	}
	keys := make([]publicKey, 0, len(public))
	for _, slot := range public {
		digest := sha256.Sum256(slot.Public[:])
		keys = append(keys, publicKey{slot.File, hex.EncodeToString(slot.Public[:]), hex.EncodeToString(digest[:])})
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
