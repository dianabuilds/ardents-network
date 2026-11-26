//go:build !linux && !darwin

package introduction

import (
	"os"
	"testing"
)

// The selected Unix-family durable root mechanism is absent on this platform,
// so every history opening must return the same explicit typed unavailable
// outcome (ADR-0129): no History, no Registry, no created root and no consumed
// fresh-root fact. This is the refusal boundary, never a weaker accepting path.
func TestUnsupportedHistoryPlatformRefusesBeforeAnyState(t *testing.T) {
	root := t.TempDir()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	if history, err := OpenHistory(root, binding); history != nil || err != errHistoryPlatformUnavailable {
		t.Fatal("unsupported OpenHistory accepted", history, err)
	}
	if history, err := InitializeHistory(root, binding, nil); history != nil || err != errHistoryPlatformUnavailable {
		t.Fatal("unsupported InitializeHistory accepted", history, err)
	}
	if registry, err := OpenReceivingRegistry(root, binding, nil); registry != nil || err != errHistoryPlatformUnavailable {
		t.Fatal("unsupported OpenReceivingRegistry accepted", registry, err)
	}
	if registry, err := OpenRetainedReceivingRegistry(root, binding); registry != nil || err != errHistoryPlatformUnavailable {
		t.Fatal("unsupported OpenRetainedReceivingRegistry accepted", registry, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("platform refusal changed native state", entries, err)
	}
}
