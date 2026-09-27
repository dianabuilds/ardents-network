package node

import (
	"errors"
	"testing"
)

func TestTerminalCleanupRetainsOneBoundedFailure(t *testing.T) {
	first := errors.New("first cleanup failure")
	second := errors.New("second cleanup failure")
	var cleanup terminalCleanup
	cleanup.record(first)
	cleanup.record(second)
	if err := cleanup.result(); !errors.Is(err, first) || errors.Is(err, second) {
		t.Fatalf("cleanup result = %v, want only first failure", err)
	}
}
