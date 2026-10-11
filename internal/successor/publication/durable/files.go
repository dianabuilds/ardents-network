package durable

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func createDirectory(path string, flush func(string) error) error {
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("publication ancestor invalid")
		}
		// Every ancestor must be a real directory, including an existing root.
		if parent := filepath.Dir(path); parent != path {
			if err = createDirectory(parent, flush); err != nil {
				return err
			}
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err = createDirectory(parent, flush); err != nil {
		return err
	}
	if err = os.Mkdir(path, 0700); err != nil {
		return err
	}
	return flush(parent)
}

func exclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(raw)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}

func resyncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

func readRegular(path string, maximum int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() > maximum {
		return nil, errors.New("publication file invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	if statErr != nil || !os.SameFile(before, info) || !info.Mode().IsRegular() {
		return nil, errors.Join(errors.New("publication file changed"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if int64(len(raw)) > maximum {
		readErr = errors.Join(readErr, errors.New("publication file exceeds bound"))
	}
	return raw, errors.Join(readErr, closeErr)
}

func scan(path string, maximum int) ([]os.DirEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, err := file.ReadDir(maximum + 1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if len(entries) > maximum {
		err = errors.Join(err, errors.New("publication inventory exceeds bound"))
	}
	return entries, errors.Join(err, file.Close())
}
