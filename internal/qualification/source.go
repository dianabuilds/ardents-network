package qualification

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type sourceFile struct {
	path   string
	digest string
}

var sourceFiles = []string{
	".dockerignore",
	".github/workflows/carrier-lab.yml",
	".github/workflows/quality.yml",
	".githooks/pre-commit",
	"AGENTS.md",
	"CONTRIBUTING.md",
	"Makefile",
	"README.md",
	"go.mod",
}

var sourceDirectories = []string{"carrier-lab", "cmd", "internal", "scripts"}

// SourceSHA256 binds maintained code, tests, and the local build/quality
// infrastructure used to qualify a Carrier Lab image.
func SourceSHA256(repositoryRoot string) (string, error) {
	root, err := canonicalRoot(repositoryRoot)
	if err != nil {
		return "", err
	}
	identities := make([]sourceFile, 0, 64)
	add := func(path string, entry fs.DirEntry) error {
		if entry != nil && entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("qualification source contains symlink %s", path)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect qualification source %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("qualification source contains non-regular file %s", path)
		}
		digest, err := fileSHA256(path)
		if err != nil {
			return fmt.Errorf("hash qualification source %s: %w", path, err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		identities = append(identities, sourceFile{path: filepath.ToSlash(relative), digest: digest})
		return nil
	}
	for _, relative := range sourceFiles {
		if err := add(filepath.Join(root, filepath.FromSlash(relative)), nil); err != nil {
			return "", err
		}
	}
	for _, directory := range sourceDirectories {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			return add(path, entry)
		})
		if err != nil {
			return "", err
		}
	}
	slices.SortFunc(identities, func(left, right sourceFile) int {
		return strings.Compare(left.path, right.path)
	})
	hash := sha256.New()
	for _, identity := range identities {
		_, _ = fmt.Fprintf(hash, "%s  %s\n", identity.digest, identity.path)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func canonicalRoot(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("repository root must be absolute")
	}
	resolved, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("repository root must be a canonical directory")
	}
	return resolved, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
