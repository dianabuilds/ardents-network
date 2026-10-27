package main

import (
	"context"
	"os"
)

// This fixed local Node-key birth creates no NodeID, family, State signer,
// membership or runtime credential. Original private output custody stays here.
func prepareForwardingNodeKey(ctx context.Context, path string, observe func(string, *os.File)) (public [32]byte, returned error) {
	return prepareForwardingPrivateKey(ctx, path, "node-key.pem", observe)
}
