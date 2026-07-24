package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func sameReplayStore(left, right string) (bool, error) {
	leftPath, leftInfo, err := canonicalReplayStorePath(left)
	if err != nil {
		return false, err
	}
	rightPath, rightInfo, err := canonicalReplayStorePath(right)
	if err != nil {
		return false, err
	}
	if leftInfo != nil && rightInfo != nil && os.SameFile(leftInfo, rightInfo) {
		return true, nil
	}
	if strings.EqualFold(leftPath, rightPath) {
		return true, nil
	}
	return false, nil
}

func canonicalReplayStorePath(path string) (string, os.FileInfo, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Stat(absolute)
	if err == nil {
		return absolute, info, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}
	resolved, err := resolveReplayStoreSymlinks(absolute)
	return resolved, nil, err
}

func resolveReplayStoreSymlinks(path string) (string, error) {
	current := filepath.Clean(path)
	for range 255 {
		resolved, changed, err := resolveFirstReplayStoreSymlink(current)
		if err != nil {
			return "", err
		}
		if !changed {
			return current, nil
		}
		current = resolved
	}
	return "", errors.New("replay store path has too many symlinks")
}

func resolveFirstReplayStoreSymlink(path string) (string, bool, error) {
	separator := string(os.PathSeparator)
	root := filepath.VolumeName(path) + separator
	parts := strings.Split(strings.TrimPrefix(path, root), separator)
	resolved := root
	for index, part := range parts {
		if part == "" {
			continue
		}
		candidate := filepath.Join(resolved, part)
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		if err != nil {
			return "", false, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			resolved = candidate
			continue
		}
		target, err := os.Readlink(candidate)
		if err != nil {
			return "", false, err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(candidate), target)
		}
		if index+1 < len(parts) {
			target = filepath.Join(target, filepath.Join(parts[index+1:]...))
		}
		return filepath.Clean(target), true, nil
	}
	return path, false, nil
}
