//go:build !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runIntroductionInstallation(context.Context, []string, io.Writer) error {
	return errors.New("Introduction input preflight requires Linux")
}
