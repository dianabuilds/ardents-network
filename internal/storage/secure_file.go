// Package storage owns secure durable primitives, transactions, permissions, and backup-safe operations.
// It does not own product schemas or lifecycle decisions.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func EnsurePrivateDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	return protectPrivatePath(dir, true)
}

func ReadPrivateFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("private state must be a regular file")
	}
	if err := validatePrivateFile(path, info); err != nil {
		return nil, false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// ReadProtectedFile upgrades a regular retained-data file to private permissions
// before reading it. It is for non-key state that may predate the private-file
// policy; key material must use ReadPrivateFile and fail closed instead.
func ReadProtectedFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("protected state must be a regular file")
	}
	if err := ProtectPrivateFile(path); err != nil {
		return nil, false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func ProtectPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("private state must be a regular file")
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return protectPrivatePath(path, false)
}

func AtomicWritePrivateFile(path string, raw []byte) (returnErr error) {
	dir := filepath.Dir(path)
	if err := EnsurePrivateDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ardents-private-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary private file: %w", err))
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if _, err := tmp.Write(raw); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Sync(); err != nil {
		return errors.Join(err, tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ProtectPrivateFile(tmpPath); err != nil {
		return err
	}
	return replacePrivateFile(tmpPath, path)
}
