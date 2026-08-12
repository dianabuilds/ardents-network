package node

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func digestNodeFixtureFiles(root string, paths []string) (string, error) {
	sort.Strings(paths)
	digest := sha256.New()
	for _, path := range paths {
		raw, err := byteio.ReadFile(path, 8<<20)
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(relative, "..") {
			return "", errors.New("node fixture input escaped its root")
		}
		fileDigest := sha256.Sum256(raw)
		_, _ = digest.Write([]byte(filepath.ToSlash(relative) + "\x00" + hex.EncodeToString(fileDigest[:]) + "\n"))
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
