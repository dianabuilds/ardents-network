package serviceconn

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

func readGeneration(path string, fallback uint64) (uint64, error) {
	if path == "" {
		return fallback, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return fallback, nil
	}
	if err != nil {
		return 0, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, 21))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(raw) > 20 {
		return 0, errors.Join(readErr, closeErr, errors.New("generation state is invalid"))
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	return value, err
}

func writeGeneration(path string, generation uint64) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(strconv.FormatUint(generation, 10)+"\n"), 0o600)
}
