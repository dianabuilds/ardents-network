package bridge_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func durableFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string][]byte)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".ardents-bridge-state-lock" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = raw
	}
	return result
}

func hexDigest(raw []byte) string {
	digest := sha256.Sum256(raw)
	const alphabet = "0123456789abcdef"
	encoded := make([]byte, 64)
	for index, value := range digest {
		encoded[index*2] = alphabet[value>>4]
		encoded[index*2+1] = alphabet[value&15]
	}
	return string(encoded)
}
