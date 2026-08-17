//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package blockedentry

import "errors"

func validateFinalConfigurationTree(string) error {
	return errors.New("final campaign configuration permissions are unsupported on this platform")
}
