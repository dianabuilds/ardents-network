//go:build browsercompat

package main

import (
	"errors"
	"os"
	"time"
)

func waitForConfiguredApplicationFault(readyPath, releasePath string, bound time.Duration) error {
	if !validPublisherApplicationPath(readyPath) || !validPublisherApplicationPath(releasePath) || readyPath == releasePath || bound <= 0 {
		return errors.New("configured Publisher Application fault control is invalid")
	}
	ready, err := os.OpenFile(readyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := ready.WriteString("ready\n"); err != nil {
		_ = ready.Close()
		return err
	}
	if err := ready.Close(); err != nil {
		return err
	}
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		raw, readErr := os.ReadFile(releasePath)
		if readErr == nil {
			if string(raw) != "inject\n" {
				return errors.New("configured Publisher Application fault release is invalid")
			}
			return nil
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	return errors.New("configured Publisher Application fault was not injected within its cycle deadline")
}
