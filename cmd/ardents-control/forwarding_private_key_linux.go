package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
)

// Physical secret generation/output only. The two fixed callers own their
// distinct Node or State purpose; no signing or authority is granted here.
func prepareForwardingPrivateKey(ctx context.Context, path, name string, observe func(string, *os.File)) (public [32]byte, returned error) {
	defer func() {
		if returned != nil {
			public = [32]byte{}
		}
	}()
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !sourceControlPath(path) || name != "node-key.pem" && name != "state-key.pem" {
		return public, errSourceControlInput
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return public, err
	}
	copy(public[:], pub)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	clear(key)
	if err != nil {
		return public, err
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	clear(der)
	defer clear(raw)
	check := func() error {
		if os.Geteuid() != 0 {
			return errSourceControlInput
		}
		return ctx.Err()
	}
	if err := persistSourceOutput(ctx, path, []sourceArtifactFile{{name, raw}}, true, check, observe); err != nil {
		return public, err
	}
	return public, check()
}
