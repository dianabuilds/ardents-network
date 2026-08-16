//go:build !linux && !windows

package blockedentry

import "errors"

func publishDirectory(_, _ string) error {
	return errors.New("atomic blocked-entry publication is unsupported on this platform")
}
