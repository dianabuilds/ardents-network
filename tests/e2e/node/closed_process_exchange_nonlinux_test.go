//go:build !linux

package state_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// The selected issuance client is Linux-only. Other platforms exercise Node
// readiness and restart, never claim a successful issuance exchange.
func prepareClosedProcessExchange(_ *testing.T, _, _ [32]byte, _ time.Time) ([32]byte, func(state.Config, bool)) {
	return [32]byte{7}, func(state.Config, bool) {}
}
