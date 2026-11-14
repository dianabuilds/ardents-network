//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runDescriptorInstallation(context.Context, []string, io.Writer) error {
	return errors.New("installed Descriptor requires the selected native Linux profile")
}
