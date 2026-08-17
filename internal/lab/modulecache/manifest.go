package modulecache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func writeSourceHashes(root, manifest string) error {
	var lines string
	for _, name := range []string{"go.mod", "go.sum"} {
		digest, _, err := hashFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		lines += digest + "  " + name + "\n"
	}
	return os.WriteFile(filepath.Join(manifest, "source.sha256"), []byte(lines), 0o600)
}

func hashFile(path string) (string, int64, error) {
	input, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	digest := sha256.New()
	size, copyErr := io.Copy(digest, input)
	return hex.EncodeToString(digest.Sum(nil)), size, errors.Join(copyErr, input.Close())
}
