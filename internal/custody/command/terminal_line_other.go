//go:build !linux && !windows

package command

import (
	"context"
	"errors"
	"os"
)

func readTerminalLine(context.Context, *os.File, bool, func() error) ([]byte, error) {
	return nil, errors.New("custody terminal reader requires Windows or Linux")
}
