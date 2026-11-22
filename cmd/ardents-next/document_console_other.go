//go:build !linux

package main

import (
	"context"
	"errors"
	"os"
)

func openDocumentCommandIO() (*os.File, *os.File, error) {
	return nil, nil, errors.New("document UI requires the selected Linux host")
}

func runPublishedLink(context.Context, string) error {
	return errors.New("document Link presentation requires the selected Linux host")
}
