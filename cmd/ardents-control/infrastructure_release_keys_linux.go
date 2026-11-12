package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// This finite physical producer creates five fresh distinct Ed25519 files.
// Each purpose-specific command owns its ceremony and public receipt. Key birth
// supplies no signature, initial pin, Release proof or runtime permission.
func prepareInfrastructureReleaseKeys(ctx context.Context, path string, observe func(string, *os.File)) (public [5][32]byte, result error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !sourceControlPath(path) {
		return public, errSourceControlInput
	}
	var files []sourceArtifactFile
	defer func() {
		for _, file := range files {
			clear(file.raw)
		}
		if result != nil {
			public = [5][32]byte{}
		}
	}()
	seen := map[[32]byte]bool{}
	for i := range public {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return public, err
		}
		copy(public[i][:], pub)
		if seen[public[i]] {
			clear(key)
			return public, errSourceControlInput
		}
		seen[public[i]] = true
		der, err := x509.MarshalPKCS8PrivateKey(key)
		clear(key)
		if err != nil {
			return public, err
		}
		raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		clear(der)
		files = append(files, sourceArtifactFile{fmt.Sprintf("release-key-%d.pem", i+1), raw})
	}
	check := func() error {
		if os.Geteuid() != 0 {
			return errSourceControlInput
		}
		return ctx.Err()
	}
	result = persistSourceOutput(ctx, path, files, true, check, observe)
	return public, result
}
