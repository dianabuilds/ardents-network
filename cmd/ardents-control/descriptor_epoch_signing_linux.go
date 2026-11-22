package main

import (
	"context"
	"os"
)

type descriptorEpochResult = nodeEpochResult[descriptorEpochPlan]

// The Descriptor genesis purpose selects only its own plan schema, decoder and
// State authority pin through the shared fixed ceremony.
func signDescriptorEpoch(ctx context.Context, args []string, observe func(string, *os.File)) (*descriptorEpochResult, error) {
	return signNodeEpoch(ctx, args, observe, maximumDescriptorEpochPlan, decodeDescriptorEpochPlan)
}
