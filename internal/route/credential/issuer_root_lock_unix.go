//go:build !windows

package credential

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type issuerRootLease struct{ file *os.File }

func acquireIssuerRootLease(root string) (issuerRootLease, error) {
	file, err := os.OpenFile(filepath.Join(root, issuerRootLockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return issuerRootLease{}, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return issuerRootLease{}, fmt.Errorf("acquire exclusive transit grant issuer lease: %w", err)
	}
	return issuerRootLease{file: file}, nil
}

func (lease issuerRootLease) release() error {
	if lease.file == nil {
		return nil
	}
	unlockErr := syscall.Flock(int(lease.file.Fd()), syscall.LOCK_UN)
	if unlockErr != nil {
		_ = lease.file.Close()
		return unlockErr
	}
	return lease.file.Close()
}
