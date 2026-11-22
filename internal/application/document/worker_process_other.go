//go:build !linux

package document

import "errors"

// RunInheritedWorker refuses platforms outside the selected installed profile.
func RunInheritedWorker(WorkerMode) error {
	return errors.New("installed text worker requires the selected Ubuntu profile")
}
