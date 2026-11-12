//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runJoinInstallation(context.Context, []string, io.Writer) error {
	return errors.New("installed JOIN requires the selected native Linux profile")
}
