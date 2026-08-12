package planfile

import (
	"errors"
	"io"
	"os"
)

// Read loads at most maximum bytes and closes the owned file before returning.
func Read(path string, maximum int64) ([]byte, error) {
	if maximum <= 0 {
		return nil, errors.New("plan file bound must be positive")
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
		return nil, errors.New("plan file exceeds its bound")
	}
	return contents, nil
}
