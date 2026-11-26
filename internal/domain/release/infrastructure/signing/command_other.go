//go:build !linux

package signing

import (
	"errors"
	"io"
)

func Run([]string, io.Writer) error {
	return errors.New("infrastructure Release signing requires Linux")
}
