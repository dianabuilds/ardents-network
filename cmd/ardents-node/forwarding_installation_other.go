//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runForwardingInstallation(context.Context, []string, io.Writer) error {
	return errors.New("installed Forwarding requires the selected native Linux profile")
}

func completedExplicitForwardingStop(error) bool { return false }
