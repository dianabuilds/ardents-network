//go:build windows

package main

import "os"

// nodeTerminationSignals keeps the portable foreground contract on Windows.
// Windows service integration is not selected for this alpha profile.
func nodeTerminationSignals() []os.Signal { return []os.Signal{os.Interrupt} }
