//go:build !windows

package durablefile

import "os"

// CreateOwnerOnlyTemp creates a private temporary file in the given directory.
func CreateOwnerOnlyTemp(root, prefix string) (*os.File, error) {
	return os.CreateTemp(root, prefix)
}

func createOwnerOnlyFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}
