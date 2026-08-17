package modulecache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func resolveLocations(options Options) (string, string, error) {
	workspace, err := exactDirectory(options.Workspace)
	if err != nil || options.Output == "" {
		return "", "", errors.Join(err, errors.New("workspace and output are required"))
	}
	target, err := filepath.Abs(options.Output)
	if err != nil {
		return "", "", err
	}
	parent, err := exactDirectory(filepath.Dir(target))
	if err != nil || filepath.Clean(parent) != filepath.Clean(filepath.Dir(target)) ||
		within(workspace, target) || within(workspace, parent) {
		return "", "", errors.Join(err, errors.New("module cache output parent must be external and unaliased"))
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return "", "", errors.New("module cache output must not already exist")
	}
	return workspace, target, nil
}

func exactDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	aliased, err := pathHasAlias(absolute)
	if err != nil || aliased {
		return "", errors.Join(err, errors.New("directory is unavailable or aliased"))
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", errors.Join(err, errors.New("path is not a directory"))
	}
	return absolute, nil
}

func pathHasAlias(path string) (bool, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}

func within(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
