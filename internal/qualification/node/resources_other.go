//go:build !linux

package node

import (
	"errors"
	"time"
)

// SampleContainerResources is available only in the Linux candidate image.
func SampleContainerResources(time.Time) ([]byte, error) {
	return nil, errors.New("node resource sampling requires a Linux container")
}

// SampleHostResources is available only in the Linux host-PID collector image.
func SampleHostResources(time.Time, string) ([]byte, error) {
	return nil, errors.New("node host resource sampling requires a Linux collector")
}
