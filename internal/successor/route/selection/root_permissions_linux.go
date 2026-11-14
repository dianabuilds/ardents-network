//go:build linux

package selection

import (
	"errors"
	"os"
	"syscall"
)

func validateOwnerOnlyRoot(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("entry state root permits group or other access")
	}
	// Match the sibling storage owners: the root must belong to the endpoint
	// user, not merely carry owner-only permission bits (audit SW-9).
	if status, ok := info.Sys().(*syscall.Stat_t); !ok || status.Uid != uint32(os.Geteuid()) {
		return errors.New("entry state root is not owned by the endpoint user")
	}
	return nil
}
