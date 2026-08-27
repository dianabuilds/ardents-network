//go:build linux

package main

import (
	"os"
	"syscall"
)

// nodeTerminationSignals are the explicit foreground and service-manager
// termination requests from which a Node must withdraw before its process exits.
func nodeTerminationSignals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }
