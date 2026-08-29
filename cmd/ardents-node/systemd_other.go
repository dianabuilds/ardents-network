//go:build !linux

package main

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func newSystemdSupervisor() (contributor.Supervisor, error) {
	return nil, errors.New("dedicated Contributor profile requires Ubuntu systemd")
}

func contributorHostRoot() string { return "" }
