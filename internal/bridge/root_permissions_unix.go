//go:build !windows

package bridge

import (
	"errors"
	"os"
)

func validateOwnerOnlyRoot(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("bridge state root permits group or other access")
	}
	return nil
}
