//go:build linux

package entry

import (
	"errors"
	"os"
)

// rootLockName is the exclusive-lease file shared by every claimed Entry
// root. The retired Invite root used the same lock name and its own
// `.ardents-entry-state-v1` marker (ADR-0106 deleted both); the closed
// Entry set root refuses any foreign population through its own marker and
// allowed-name inspection.
const rootLockName = ".ardents-entry-state-lock"

// inspectRoot creates or validates the owned directory for one Entry root.
func inspectRoot(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("entry state root is not an owned directory")
	}
	return nil
}

// validateRootPermissions enforces the owner-only access floor for one
// claimed Entry root through the platform owner check.
func validateRootPermissions(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return validateOwnerOnlyRoot(root, info)
}

// writeExclusive creates one new owner-only file and durably writes its
// exact bytes, refusing any pre-existing target.
func writeExclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
