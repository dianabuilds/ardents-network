//go:build windows

package admission

import (
	"errors"
	"os"
)

// OpenConsoleFile reports unsupported console access on Windows.
func OpenConsoleFile(*os.File) (*os.File, error) {
	return nil, errors.New("interactive admission requires a supported host")
}
