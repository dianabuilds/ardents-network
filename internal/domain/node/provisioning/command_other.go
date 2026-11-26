//go:build !linux

package provisioning

import (
	"context"
	"errors"
	"io"
)

func Run([]string, io.Writer) error {
	return errors.New("infrastructure provisioning requires Linux")
}

func PrepareInitialNetworkKeys(context.Context, string, string) ([]byte, error) {
	return nil, errors.New("initial Network key preparation requires Linux")
}
