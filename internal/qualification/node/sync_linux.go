//go:build linux

package node

import (
	"errors"
	"os"
)

func syncNodeDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}
