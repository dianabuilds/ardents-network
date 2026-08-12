//go:build linux

package node

import (
	"errors"

	"golang.org/x/sys/unix"
)

func lowerNodeNofile() error {
	var current unix.Rlimit
	if err := unix.Prlimit(1, unix.RLIMIT_NOFILE, nil, &current); err != nil {
		return err
	}
	if current.Cur < 8 || current.Max < 8 {
		return errors.New("Node descriptor limit is below the fault-injection boundary")
	}
	limit := unix.Rlimit{Cur: 8, Max: 8}
	return unix.Prlimit(1, unix.RLIMIT_NOFILE, &limit, nil)
}
