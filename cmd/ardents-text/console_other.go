//go:build !linux

package main

import (
	"context"
	"errors"
	"os"
)

func openTextCommandIO() (*os.File, *os.File, error) {
	return nil, nil, errors.New("text UI requires the selected Linux host")
}

func runPublishedLink(context.Context, string) error {
	return errors.New("text Link presentation requires the selected Linux host")
}
