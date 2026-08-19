//go:build live

package network_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func blockedStateTreeHash(t *testing.T, root string) []byte {
	t.Helper()
	hash := sha256.New()
	hash.Write([]byte("ardents-h3-state-tree-v1\x00"))
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		hash.Write([]byte("absent"))
		return []byte(hex.EncodeToString(hash.Sum(nil)))
	} else if err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("state tree contains a non-regular entry")
		}
		name := []byte(filepath.ToSlash(relative))
		_ = binary.Write(hash, binary.BigEndian, uint32(len(name)))
		hash.Write(name)
		if info.IsDir() {
			hash.Write([]byte{'d'})
			return nil
		}
		hash.Write([]byte{'f'})
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = binary.Write(hash, binary.BigEndian, uint64(len(raw)))
		hash.Write(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return []byte(hex.EncodeToString(hash.Sum(nil)))
}
