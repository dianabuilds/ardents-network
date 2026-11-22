package main

import (
	"context"
	"os"
)

type joinEpochResult = nodeEpochResult[joinEpochPlan]

// The JOIN genesis purpose selects only its own plan schema, decoder and State
// authority pin through the shared fixed ceremony.
func signJoinEpoch(ctx context.Context, args []string, observe func(string, *os.File)) (*joinEpochResult, error) {
	return signNodeEpoch(ctx, args, observe, maximumJoinEpochPlan, decodeJoinEpochPlan)
}
