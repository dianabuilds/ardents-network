//go:build !linux

package installation

import (
	"context"
	"errors"
)

func observePlatform(context.Context) error {
	return errors.New("protected installation observation requires Ubuntu24.04 amd64")
}

func readInstalledFile(string, int64) ([]byte, error) {
	return nil, errors.New("protected installation file observation requires Ubuntu24.04 amd64")
}

func observeBinding(checkedBinding) error {
	return errors.New("protected installation binding observation requires Ubuntu24.04 amd64")
}
