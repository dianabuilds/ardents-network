//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runInitialInfrastructureRelease(context.Context, []string, io.Writer) error {
	return errors.New("initial infrastructure Release requires Linux/amd64")
}
