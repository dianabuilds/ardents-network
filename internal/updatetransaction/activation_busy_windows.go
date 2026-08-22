//go:build windows

package updatetransaction

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isActivationBusy admits only the two Windows kernel refusals that mean a
// competing handle or byte-range lock prevented current replacement. It does
// not classify access, path, reparse, or durability failures as contention.
func isActivationBusy(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
