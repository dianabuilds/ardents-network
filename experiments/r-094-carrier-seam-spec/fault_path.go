//go:build ignore

package main

import (
	"context"
	"errors"
	"os"
	"time"
)

const faultResumePath = "/tmp/ardents-r094-path-resume"

func faultPrepareResume(path string) error {
	if path == "" {
		return nil
	}
	if path != faultResumePath {
		return errors.New("fault resume path is outside the lab contract")
	}
	_, err := os.Lstat(path)
	if err == nil {
		return errors.New("fault resume path is already occupied")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func faultWaitForResume(ctx context.Context, path string) error {
	for {
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != 0 {
				return errors.New("fault resume control is not an empty regular file")
			}
			return os.Remove(path)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
