//go:build !linux

package service_test

import "os"

// Windows has no equivalent per-child service termination signal in this test
// harness. Its compatibility run retains explicit forced cleanup instead.
func referenceC2RequestProductNodeShutdown(_ *os.Process) (bool, error) { return false, nil }
