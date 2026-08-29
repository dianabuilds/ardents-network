package node

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"
)

const ContributorClockObservationInterval = 500 * time.Millisecond

type clockObservationConfig struct {
	now       func() time.Time
	newTicker func(time.Duration) (<-chan time.Time, func())
}

// StartContributorClockObservation keeps the installed profile's clock marker
// current until its returned stop function has joined the owner goroutine.
func StartContributorClockObservation(parent context.Context, path string, interval time.Duration) (func() error, error) {
	return startClockObservation(parent, path, interval, clockObservationConfig{
		now: time.Now,
		newTicker: func(interval time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(interval)
			return ticker.C, ticker.Stop
		},
	})
}

func startClockObservation(parent context.Context, path string, interval time.Duration, config clockObservationConfig) (func() error, error) {
	if parent == nil || path == "" || interval <= 0 || interval >= 2*time.Second || config.now == nil || config.newTicker == nil {
		return nil, errors.New("contributor clock observation owner is invalid")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("contributor clock observation must be one regular file")
	}
	refresh := func() error {
		now := config.now()
		return os.Chtimes(path, now, now)
	}
	if err := refresh(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan error, 1)
	go func() {
		ticks, stopTicker := config.newTicker(interval)
		defer stopTicker()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticks:
				if err := refresh(); err != nil {
					done <- err
					return
				}
			}
		}
	}()
	var once sync.Once
	var stopErr error
	return func() error {
		once.Do(func() {
			cancel()
			stopErr = <-done
		})
		return stopErr
	}, nil
}
