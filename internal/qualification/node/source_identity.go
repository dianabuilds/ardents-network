package node

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type nodeSourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type nodeSourceReceipt struct {
	Schema string           `json:"schema"`
	Digest string           `json:"digest"`
	Files  []nodeSourceFile `json:"files"`
}

func captureNodeSourceIdentity(composeFile, evidenceRoot string) (string, error) {
	repository := filepath.Clean(filepath.Join(filepath.Dir(composeFile), "..", "..", ".."))
	if _, err := byteio.ReadFile(filepath.Join(repository, "go.mod"), 64<<10); err != nil {
		return "", errors.New("node campaign cannot locate its source root")
	}
	paths := []string{filepath.Join(repository, "go.mod"), filepath.Join(repository, "go.sum"),
		filepath.Join(repository, ".dockerignore"), composeFile,
		filepath.Join(filepath.Dir(composeFile), "Dockerfile")}
	for _, directory := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repository, directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(paths)
	combined := sha256.New()
	files := make([]nodeSourceFile, 0, len(paths))
	for _, path := range paths {
		raw, err := byteio.ReadFile(path, 2<<20)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(raw)
		relative, err := filepath.Rel(repository, path)
		if err != nil || strings.HasPrefix(relative, "..") {
			return "", errors.New("node source identity escaped its root")
		}
		name := filepath.ToSlash(relative)
		encoded := hex.EncodeToString(digest[:])
		files = append(files, nodeSourceFile{Path: name, SHA256: encoded})
		_, _ = combined.Write([]byte(name + "\x00" + encoded + "\n"))
	}
	digest := hex.EncodeToString(combined.Sum(nil))
	receipt := nodeSourceReceipt{Schema: "ardents-h3-node-source-v1", Digest: digest, Files: files}
	if err := byteio.WriteJSON(filepath.Join(evidenceRoot, "source-identity.json"), receipt, 1<<20); err != nil {
		return "", err
	}
	return digest, nil
}
