package keyset

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// Prepare creates one original set of Release key files and returns its public inventory.
func Prepare(ctx context.Context, path string) (Inventory, error) {
	return prepare(ctx, path, nil)
}

// This finite physical producer creates five fresh distinct Ed25519 files.
// Each purpose-specific command owns its ceremony and public receipt. Key birth
// supplies no signature, initial pin, Release proof or runtime permission.
func prepare(ctx context.Context, path string, observe func(string, *os.File)) (public Inventory, result error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || !artifactio.AbsolutePath(path) {
		return public, artifactio.ErrInput
	}
	var files []artifactio.Content
	defer func() {
		for _, file := range files {
			clear(file.Data)
		}
		if result != nil {
			public = Inventory{}
		}
	}()
	seen := map[[32]byte]bool{}
	for i := range public {
		pub, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return public, err
		}
		var slot Slot
		slot.File = fmt.Sprintf("release-key-%d.pem", i+1)
		copy(slot.Public[:], pub)
		if seen[slot.Public] {
			clear(key)
			return public, artifactio.ErrInput
		}
		seen[slot.Public] = true
		der, err := x509.MarshalPKCS8PrivateKey(key)
		clear(key)
		if err != nil {
			return public, err
		}
		raw := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		clear(der)
		files = append(files, artifactio.Content{Name: slot.File, Data: raw})
		public[i] = slot
	}
	check := func() error {
		if os.Geteuid() != 0 {
			return artifactio.ErrInput
		}
		return ctx.Err()
	}
	result = artifactio.PublishPrivate(ctx, path, files, check, observe)
	return public, result
}
