//go:build !windows

package selection

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

type rootLease struct{ file *os.File }

func acquireRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR|os.O_CREATE)
}

func acquireFreshRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR|os.O_CREATE|os.O_EXCL)
}

// Retained selection history must never regain a missing lease file by opening.
func acquireRetainedRootLease(root string) (rootLease, error) {
	return openRootLease(root, os.O_RDWR)
}

// openRootLease retains the lease only after the shared strict identity check
// (regular file, single link), matching the sibling duty and State durable
// owners so a substituted lease inode can never become the Entry fence (G1).
func openRootLease(root string, flags int) (rootLease, error) {
	file, err := durablefile.OpenLease(filepath.Join(root, rootLockName), flags|syscall.O_NOFOLLOW, "", "acquire exclusive Entry state lease")
	if err != nil {
		return rootLease{}, err
	}
	return rootLease{file: file}, nil
}

func (lease rootLease) release() error {
	return durablefile.ReleaseLease(lease.file)
}
