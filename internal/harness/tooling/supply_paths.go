package tooling

import (
	"errors"
	"os"
	"path/filepath"
)

func canonicalDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(absolute)
	if err := requireCanonicalDirectory(clean); err != nil {
		return "", err
	}
	return clean, nil
}

func canonicalRegularFile(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("path must be absolute and clean")
	}
	if err := requireNoSymlinkComponents(path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("path must name a regular file")
	}
	return path, nil
}
