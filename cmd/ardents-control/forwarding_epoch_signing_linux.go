package main

import (
	"context"
	"os"
)

type forwardingEpochResult = nodeEpochResult[forwardingEpochPlan]

// The Forwarding genesis purpose selects only its own plan schema, decoder and
// State authority pin through the shared fixed ceremony.
func signForwardingEpoch(ctx context.Context, args []string, observe func(string, *os.File)) (*forwardingEpochResult, error) {
	return signNodeEpoch(ctx, args, observe, maximumForwardingEpochPlan, decodeForwardingEpochPlan)
}
