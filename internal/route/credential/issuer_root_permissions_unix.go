//go:build !windows

package credential

import (
	"errors"
	"os"
)

func validateIssuerRootPermissions(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("transit grant issuer root permits group or other access")
	}
	return nil
}
