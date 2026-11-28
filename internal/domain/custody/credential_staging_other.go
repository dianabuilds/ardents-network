//go:build !windows

package custody

import (
	"errors"
	"os"
)

func createCredentialStaging(parent, prefix string, _ bool) (*os.File, error) {
	return os.CreateTemp(parent, prefix)
}

func retainCredentialStaging(_ bool) bool { return false }

func closeCredentialStaging(file *os.File, path string, _, open bool) error {
	var closeErr error
	if open {
		closeErr = file.Close()
	}
	removeErr := os.Remove(path)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(closeErr, removeErr)
}

func openCredentialFile(path string, _ bool) (*os.File, error) {
	return os.Open(path)
}

func sameCredentialFile(file *os.File, path string) bool {
	opened, openErr := file.Stat()
	named, nameErr := os.Lstat(path)
	return openErr == nil && nameErr == nil && os.SameFile(opened, named)
}
