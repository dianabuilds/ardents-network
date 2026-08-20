package stage6evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func writeJSON(root, relative, schema string, value any, jsonl bool) (artifact, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return artifact{}, err
	}
	if jsonl {
		raw = append(raw, '\n')
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return artifact{}, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return artifact{}, err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return artifact{}, err
	}
	if closeErr != nil {
		return artifact{}, closeErr
	}
	digest := sha256.Sum256(raw)
	return artifact{Path: filepath.ToSlash(relative), Schema: schema, Size: int64(len(raw)),
		SHA256: hex.EncodeToString(digest[:])}, nil
}

func prepareRoots(base string) (private, manifest, evidence string, err error) {
	info, statErr := os.Lstat(base)
	if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", errors.New("S6E1 base root is invalid")
	}
	entries, readErr := os.ReadDir(base)
	if readErr != nil || len(entries) != 0 {
		return "", "", "", errors.New("S6E1 base root must be empty")
	}
	private, manifest, evidence = filepath.Join(base, "private"), filepath.Join(base, "manifest"), filepath.Join(base, "evidence")
	for _, root := range []string{private, manifest, evidence} {
		if mkdirErr := os.Mkdir(root, 0o700); mkdirErr != nil {
			return "", "", "", mkdirErr
		}
	}
	return private, manifest, evidence, nil
}

func executableDigest(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("S6E1 executable is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	opened, statErr := file.Stat()
	raw, readErr := io.ReadAll(file)
	after, afterErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || readErr != nil || afterErr != nil || closeErr != nil ||
		!os.SameFile(before, opened) || !os.SameFile(opened, after) ||
		opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return "", errors.New("S6E1 executable changed during read")
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
