//go:build !linux && !darwin

package targetlink

import (
	"errors"
	"os"
)

func openPrivateFile(string) (*os.File, error) {
	return nil, errors.New("private Target Link requires a supported host")
}
