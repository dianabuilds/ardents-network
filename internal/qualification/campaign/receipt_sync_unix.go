//go:build !windows

package campaign

import (
	"errors"
	"os"
)

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func publishReceipt(pending, final, attemptRoot string) error {
	if err := os.Rename(pending, final); err != nil {
		return err
	}
	return syncDirectory(attemptRoot)
}
