//go:build linux

package main

import (
	"context"
	"errors"
	"os"
	"time"
)

func runPublishedLink(ctx context.Context, socket string) error {
	output, err := openTextPresentationDescriptor("/proc/self/fd/1", os.Stdout, os.O_WRONLY)
	if err != nil {
		return err
	}
	if err := output.SetWriteDeadline(time.Time{}); err != nil {
		return errors.Join(err, output.Close())
	}
	return showPublishedLink(ctx, socket, output)
}
