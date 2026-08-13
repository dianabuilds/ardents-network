//go:build !linux

package node

import (
	"errors"
	"time"
)

func sampleContainerResources(time.Time) ([]byte, error) {
	return nil, errors.New("node resource sampling requires a Linux container")
}

func sampleHostResources(time.Time, string) ([]byte, error) {
	return nil, errors.New("node host resource sampling requires a Linux collector")
}
