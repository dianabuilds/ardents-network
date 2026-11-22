package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

// writeReleaseKeyReceipt reports the five public keys after the selected
// preparation command has finished writing its private files.
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
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}
