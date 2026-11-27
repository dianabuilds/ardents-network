//go:build !linux

package console

import (
	"errors"
	"os"
)

// Open reports the unavailable local presentation boundary.
func Open() (*os.File, *os.File, error) {
	return nil, nil, errors.New("document UI requires the selected Linux host")
}
