// Package storage owns secure durable primitives, transactions, permissions, and backup-safe operations.
// It does not own product schemas or lifecycle decisions.
package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var privateCreateMu sync.Mutex

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

// ValidatePrivateDir verifies an existing key-custody directory without
// changing its permissions or ACL.
func ValidatePrivateDir(dir string) error {
	if dir == "" {
		dir = "."
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private state parent must be a directory")
	}
	return validatePrivateDirectory(dir, info)
}

// ValidateStrictPrivateFile verifies an existing private regular file without
// opening it or changing its permissions. Callers that hand the path to a
// database library use this to fail closed before the library can follow a
// symlink or repair an exposed file.
func ValidateStrictPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("private state must be a regular file")
	}
	return validateStrictPrivateFile(path, info)
}

func ensurePrivateCreateDir(dir string) error {
	if dir == "" {
		dir = "."
	}
	_, err := os.Lstat(dir)
	if err == nil {
		return ValidatePrivateDir(dir)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	if err := protectPrivatePath(dir, true); err != nil {
		return err
	}
	return ValidatePrivateDir(dir)
}

func ReadPrivateFile(path string) ([]byte, bool, error) {
	return ReadPrivateFileBounded(path, 0)
}

// ReadPrivateFileBounded reads a private regular file while enforcing an
// optional positive size limit before allocating caller-controlled content.
func ReadPrivateFileBounded(path string, maxBytes int64) ([]byte, bool, error) {
	return readPrivateFileBounded(path, maxBytes, false)
}

// ReadStrictPrivateFileBounded refuses an existing file whose protection is
// not already private. It is the key-custody read path; unlike retained-state
// migration it never repairs permissions or ACLs after possible exposure.
func ReadStrictPrivateFileBounded(path string, maxBytes int64) ([]byte, bool, error) {
	return readPrivateFileBounded(path, maxBytes, true)
}

func readPrivateFileBounded(path string, maxBytes int64, strict bool) ([]byte, bool, error) {
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
	validate := validatePrivateFile
	if strict {
		validate = validateStrictPrivateFile
	}
	if err := validate(path, info); err != nil {
		return nil, false, err
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, false, fmt.Errorf("private state exceeds size limit")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	reader := io.Reader(file)
	if maxBytes > 0 {
		reader = io.LimitReader(file, maxBytes+1)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, false, err
	}
	if maxBytes > 0 && int64(len(raw)) > maxBytes {
		return nil, false, fmt.Errorf("private state exceeds size limit")
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
	if err := ProtectPrivateFile(tmpPath); err != nil {
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
	return replacePrivateFile(tmpPath, path)
}

// AtomicCreatePrivateFile publishes a new private file without replacing an
// existing path. Concurrent creators therefore have exactly one winner.
func AtomicCreatePrivateFile(path string, raw []byte) (returnErr error) {
	privateCreateMu.Lock()
	defer privateCreateMu.Unlock()
	dir := filepath.Dir(path)
	if err := ensurePrivateCreateDir(dir); err != nil {
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
	if err := ProtectPrivateFile(tmpPath); err != nil {
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
	return publishPrivateFileNoReplace(tmpPath, path)
}
