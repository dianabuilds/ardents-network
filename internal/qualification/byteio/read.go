package byteio

import (
	"errors"
	"io"
	"os"
)

// ReadFile reads at most maximum bytes before rejecting the file.
func ReadFile(path string, maximum int64) ([]byte, error) {
	if maximum < 0 {
		return nil, errors.New("file byte bound is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(contents)) > maximum {
		return nil, errors.New("file exceeds its byte bound")
	}
	return contents, nil
}

// ReadDirectory enumerates at most maximum entries before rejecting the directory.
func ReadDirectory(path string, maximum int) ([]os.DirEntry, error) {
	if maximum < 0 {
		return nil, errors.New("directory entry bound is invalid")
	}
	directory, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := directory.ReadDir(maximum + 1)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) > maximum {
		return nil, errors.New("directory exceeds its entry bound")
	}
	return entries, nil
}
