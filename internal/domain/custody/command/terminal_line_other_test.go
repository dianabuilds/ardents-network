//go:build !linux && !windows

package command

import (
	"context"
	"testing"
)

func TestUnsupportedTerminalLineRefusesBeforeReady(t *testing.T) {
	for _, secret := range []bool{false, true} {
		ready := false
		value, err := readTerminalLine(context.Background(), nil, secret, func() error { ready = true; return nil })
		if err == nil || value != nil || ready {
			t.Fatalf("readTerminalLine(secret=%t): value=%v error=%v ready=%t; want refusal without terminal effects", secret, value, err, ready)
		}
	}
}
