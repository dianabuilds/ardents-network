package spending

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// The retained lease file distinguishes an opened root from a fresh one.
// Its absence cannot authorize replacing part of an existing spend history.
func openClosedSpendLeaseFile(path string) (*os.File, bool, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if !errors.Is(err, os.ErrNotExist) {
		return file, false, err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, false, err
	}
	entries, readErr := directory.Readdirnames(1)
	closeErr := directory.Close()
	if len(entries) != 0 {
		return nil, false, errors.Join(errors.New("closed spend root is missing its retained lease"), closeErr)
	}
	if !errors.Is(readErr, io.EOF) || closeErr != nil {
		return nil, false, errors.Join(readErr, closeErr)
	}
	file, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	return file, err == nil, err
}
