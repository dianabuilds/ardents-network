package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// A separate fixed initial Source Release ceremony. No Endpoint receipt or
// fixture key is reused. This produces no signature or trust/independence claim.
func runSourceReleaseKeys(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	public, err := prepareSourceReleaseKeys(ctx, args[0], nil)
	if err != nil {
		return &sourceControlFailure{err}
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
	}{"source.prepare-release-keys", 3, false, keys})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}

func prepareSourceReleaseKeys(ctx context.Context, path string, observe func(string, *os.File)) (public [5][32]byte, result error) {
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
