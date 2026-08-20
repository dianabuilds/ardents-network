package releasedecision

import (
	"errors"
	"fmt"
	"io"
	"os"
)

func readBoundedFloorFile(path string, bound int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("releasedecision: %s is not a regular file", path)
	}
	if before.Size() > bound {
		return nil, fmt.Errorf("releasedecision: %s exceeds the bound", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	after, statErr := file.Stat()
	if statErr != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("releasedecision: state file changed while it was opened"), statErr, file.Close())
	}
	limited := &io.LimitedReader{R: file, N: bound + 1}
	buffer, readErr := io.ReadAll(limited)
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(buffer)) > bound {
		return nil, fmt.Errorf("releasedecision: %s exceeds the bound", path)
	}
	return buffer, nil
}

func readFloorStoreDirectory(path string, bound int) (result []os.DirEntry, resultErr error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("releasedecision: state directory is not a directory")
	}
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	after, statErr := directory.Stat()
	if statErr != nil || !after.IsDir() || !os.SameFile(before, after) {
		return nil, errors.Join(errors.New("releasedecision: state directory changed while it was opened"), statErr)
	}
	entries := make([]os.DirEntry, 0, bound)
	for {
		batch, readErr := directory.ReadDir(8)
		entries = append(entries, batch...)
		if len(entries) > bound {
			return nil, fmt.Errorf("releasedecision: %s exceeds the entry bound", path)
		}
		if errors.Is(readErr, io.EOF) {
			return entries, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("releasedecision: state entry is not a regular file")
	}
	return nil
}
