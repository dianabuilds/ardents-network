//go:build windows

package console

import (
	"errors"
	"os"
)

// openFile reports unsupported console access on Windows.
func openFile(*os.File) (*os.File, error) {
	return nil, errors.New("interactive admission requires a supported host")
}
