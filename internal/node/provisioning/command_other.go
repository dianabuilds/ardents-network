//go:build !linux

package provisioning

import (
	"errors"
	"io"
)

func Run([]string, io.Writer) error {
	return errors.New("infrastructure provisioning requires Linux")
}
