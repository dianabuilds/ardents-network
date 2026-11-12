package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os/signal"
	"syscall"
	"time"
)

// A separate fixed initial Forwarding Release ceremony. No Endpoint receipt or
// fixture key is reused. This produces no signature or trust/independence claim.
func runForwardingReleaseKeys(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareInfrastructureReleaseKeys(ctx, args[0], nil)
	if err != nil {
		return &forwardingControlFailure{err}
	}
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
	}{"forwarding.prepare-release-keys", 3, false, keys})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}
