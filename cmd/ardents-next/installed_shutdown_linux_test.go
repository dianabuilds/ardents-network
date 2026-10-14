//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestInstalledShutdownRetainsAdditionalFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	failure := errors.New("original physical cleanup failed")
	if installedEndpointFailure(ctx, context.Canceled) != 1 {
		t.Fatal("foreign cancellation became requested shutdown")
	}
	cancel()
	for _, err := range []error{nil, context.Canceled, fmt.Errorf("original observation: %w", context.Canceled), errors.Join(context.Canceled, context.Canceled)} {
		if installedEndpointFailure(ctx, err) != 0 {
			t.Fatal("joined requested cancellation became failed invocation", err)
		}
	}
	for _, err := range []error{failure, context.DeadlineExceeded, errors.Join(context.Canceled, failure), fmt.Errorf("close: %w", errors.Join(failure, context.Canceled))} {
		if installedEndpointFailure(ctx, err) != 1 {
			t.Fatal("requested shutdown hid original failure", err)
		}
	}
}
