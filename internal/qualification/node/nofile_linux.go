//go:build linux

package node

import (
	"errors"

	"golang.org/x/sys/unix"
)

const (
	nodeProcessPID           = 1
	emfileDescriptorBoundary = 8
)

func lowerNodeNofile() error {
	var current unix.Rlimit
	if err := unix.Prlimit(nodeProcessPID, unix.RLIMIT_NOFILE, nil, &current); err != nil {
		return err
	}
	if current.Cur < emfileDescriptorBoundary || current.Max < emfileDescriptorBoundary {
		return errors.New("node descriptor limit is below the fault-injection boundary")
	}
	limit := uint64(emfileDescriptorBoundary)
	return unix.Prlimit(nodeProcessPID, unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: limit, Max: limit}, nil)
}
