//go:build !linux

package replacement

import (
	"errors"
	"os"
)

func acquireLock(string) (*os.File, error) {
	return nil, errors.New("endpoint replacement is available only on Linux")
}

func releaseLock(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Close()
}
