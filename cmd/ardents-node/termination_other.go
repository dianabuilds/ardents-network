//go:build !linux && !windows

package main

import "os"

func nodeTerminationSignals() []os.Signal { return []os.Signal{os.Interrupt} }
