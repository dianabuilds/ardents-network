//go:build !linux && !windows

package main

import (
	"context"
	"errors"
	"os"
)

func readTerminalLine(context.Context, *os.File, bool, func()) ([]byte, error) {
	return nil, errors.New("custody terminal reader requires Windows or Linux")
}
