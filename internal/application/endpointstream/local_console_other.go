//go:build !linux

package endpointstream

import (
	"errors"
	"os"
)

// OpenLocalConsole reports the unavailable local presentation boundary.
func OpenLocalConsole() (*os.File, *os.File, error) {
	return nil, nil, errors.New("document UI requires the selected Linux host")
}
