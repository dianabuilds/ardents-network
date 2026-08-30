package custody

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const vaultLockName = ".ardents-custody-vault.lock"

func prepareVaultLock(root string) error {
	path := filepath.Join(root, vaultLockName)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if createErr != nil {
			return fmt.Errorf("create custody Vault lock: %w", createErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("close custody Vault lock: %w", closeErr)
		}
		return syncDirectory(root)
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() != 0 || info.Mode()&os.ModeSymlink != 0 {
		return ErrInvalid
	}
	return nil
}
