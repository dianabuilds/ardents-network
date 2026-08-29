package main

import (
	"context"
	"errors"
	"os"
	"time"
)

const contributorClockObservationInterval = 500 * time.Millisecond

func startContributorClockObservation(parent context.Context, path string, interval time.Duration) (func() error, error) {
	if parent == nil || path == "" || interval <= 0 || interval >= 2*time.Second {
		return nil, errors.New("Contributor clock observation owner is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("Contributor clock observation must be one regular file")
	}
	refresh := func() error {
		now := time.Now()
		return os.Chtimes(path, now, now)
	}
	if err := refresh(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticker.C:
				if err := refresh(); err != nil {
					done <- err
					return
				}
			}
		}
	}()
	var stopped bool
	return func() error {
		if stopped {
			return nil
		}
		stopped = true
		cancel()
		return <-done
	}, nil
}
