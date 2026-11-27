//go:build !linux && !darwin

package endpointstream

import (
	"errors"
	"os"
)

func openPrivateLinkFile(string) (*os.File, error) {
	return nil, errors.New("private Target Link requires a supported host")
}
