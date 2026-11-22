package main

import (
	"errors"
	"io"
	"os"
)

func readAuthorityRequest(path string, limit int64, kind string) ([]byte, error) {
	invalid := errors.New(kind + " file is invalid")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return nil, invalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || opened.Size() != info.Size() {
		return nil, errors.Join(invalid, err, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || int64(len(raw)) != opened.Size() {
		return nil, errors.Join(readErr, closeErr, invalid)
	}
	return raw, nil
}
