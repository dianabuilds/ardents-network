//go:build linux

package main

import (
	"os"
	"syscall"
	"testing"
)

func TestNodeTerminationSignalsIncludeSIGTERM(t *testing.T) {
	found := false
	for _, candidate := range nodeTerminationSignals() {
		if candidate == syscall.SIGTERM {
			found = true
		}
	}
	if !found {
		t.Fatalf("Node termination signals = %v, want SIGTERM", nodeTerminationSignals())
	}
	if len(nodeTerminationSignals()) == 0 || nodeTerminationSignals()[0] != os.Interrupt {
		t.Fatalf("Node termination signals must retain foreground interrupt")
	}
}
