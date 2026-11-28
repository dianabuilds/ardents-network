//go:build !linux

package main

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

// runInheritedWorker refuses platforms outside the selected installed profile.
func runInheritedWorker(document.WorkerMode) error {
	return errors.New("installed document worker requires the selected Ubuntu profile")
}
