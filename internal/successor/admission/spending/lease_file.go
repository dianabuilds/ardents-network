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

// Retained opening obtains the actual original file without any creating flag.
// A separate stat-before-ordinary-Open would still allow erased history to be
// initialized. Detached existence is not retained native custody.
func openRetainedClosedSpendLeaseFile(path string) (*os.File, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() != 0 {
		return nil, errors.New("retained closed spend lease is invalid")
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	after, err := file.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || namedErr != nil || !os.SameFile(before, after) || !os.SameFile(before, named) || after.Mode() != before.Mode() || after.Size() != 0 {
		return nil, errors.Join(errors.New("retained closed spend lease changed"), err, namedErr, file.Close())
	}
	return file, nil
}
