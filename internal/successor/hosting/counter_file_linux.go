//go:build linux

package hosting

import (
	"errors"
	"io"
	"os"
)

func boundedFile(path string, maximum int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(f, maximum+1))
	err = errors.Join(readErr, f.Close())
	if int64(len(body)) > maximum {
		return nil, ErrUnavailable
	}
	return body, err
}
