package node

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
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

func captureNodeSourceIdentity(composeFile, evidenceRoot string) (string, string, error) {
	repository := filepath.Clean(filepath.Join(filepath.Dir(composeFile), "..", "..", ".."))
	if _, err := byteio.ReadFile(filepath.Join(repository, "go.mod"), 64<<10); err != nil {
		return "", "", errors.New("node campaign cannot locate its source root")
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
			return "", "", err
		}
	}
	sort.Strings(paths)
	snapshot := filepath.Join(evidenceRoot, "source-snapshot")
	if err := os.Mkdir(snapshot, 0o700); err != nil {
		return "", "", err
	}
	combined := sha256.New()
	files := make([]nodeSourceFile, 0, len(paths))
	for _, path := range paths {
		raw, err := byteio.ReadFile(path, 2<<20)
		if err != nil {
			return "", "", err
		}
		digest := sha256.Sum256(raw)
		relative, err := filepath.Rel(repository, path)
		if err != nil || strings.HasPrefix(relative, "..") {
			return "", "", errors.New("node source identity escaped its root")
		}
		destination := filepath.Join(snapshot, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return "", "", err
		}
		if err := os.WriteFile(destination, raw, 0o400); err != nil {
			return "", "", err
		}
		name := filepath.ToSlash(relative)
		encoded := hex.EncodeToString(digest[:])
		files = append(files, nodeSourceFile{Path: name, SHA256: encoded})
		_, _ = combined.Write([]byte(name + "\x00" + encoded + "\n"))
	}
	digest := hex.EncodeToString(combined.Sum(nil))
	receipt := nodeSourceReceipt{Schema: "ardents-h3-node-source-v1", Digest: digest, Files: files}
	if err := byteio.WriteJSON(filepath.Join(evidenceRoot, "source-identity.json"), receipt, 1<<20); err != nil {
		return "", "", err
	}
	return digest, snapshot, nil
}

func validateNodeSourceIdentity(evidenceRoot, snapshotRoot, expectedDigest string) error {
	raw, err := byteio.ReadFile(filepath.Join(evidenceRoot, "source-identity.json"), 1<<20)
	if err != nil {
		return err
	}
	var receipt nodeSourceReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "ardents-h3-node-source-v1" || receipt.Digest != expectedDigest || len(receipt.Files) == 0 {
		return errors.New("node source identity receipt is invalid")
	}
	combined := sha256.New()
	owned := make(map[string]bool, len(receipt.Files))
	previous := ""
	for _, file := range receipt.Files {
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(file.Path)))
		if clean != file.Path || filepath.IsAbs(file.Path) || strings.HasPrefix(clean, "../") || clean <= previous {
			return errors.New("node source identity path set is invalid")
		}
		content, readErr := byteio.ReadFile(filepath.Join(snapshotRoot, filepath.FromSlash(file.Path)), 2<<20)
		if readErr != nil {
			return readErr
		}
		digest := sha256.Sum256(content)
		encoded := hex.EncodeToString(digest[:])
		if encoded != file.SHA256 {
			return errors.New("node source snapshot changed after preflight")
		}
		_, _ = combined.Write([]byte(file.Path + "\x00" + encoded + "\n"))
		owned[file.Path], previous = true, file.Path
	}
	if hex.EncodeToString(combined.Sum(nil)) != expectedDigest {
		return errors.New("node source snapshot digest is invalid")
	}
	seen := 0
	err = filepath.WalkDir(snapshotRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("node source snapshot contains a link")
		}
		if entry.IsDir() {
			return nil
		}
		relative, relativeErr := filepath.Rel(snapshotRoot, path)
		if relativeErr != nil || !owned[filepath.ToSlash(relative)] {
			return errors.New("node source snapshot contains an unsealed file")
		}
		seen++
		return nil
	})
	if err != nil || seen != len(receipt.Files) {
		return errors.Join(err, errors.New("node source snapshot file set is invalid"))
	}
	return nil
}
