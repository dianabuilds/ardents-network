//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runInstalledSource(context.Context, string, io.Writer) error {
	return errors.New("installed Source requires the selected native Linux profile")
}

func runSourceInstallation(context.Context, []string, io.Writer) error {
	return errors.New("source installation requires the selected native Linux profile")
}
