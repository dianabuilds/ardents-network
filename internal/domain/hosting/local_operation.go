package hosting

import (
	"context"
	"time"
)

// ExecuteLocal initializes a budget or obtains a joined observation.
func ExecuteLocal(ctx context.Context, operation string, plan CommandPlan) (SampleResult, error) {
	if ctx == nil {
		return SampleResult{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return SampleResult{}, err
	}
	switch operation {
	case "initialize":
		return SampleResult{}, Initialize(plan.Root, plan.Policy)
	case "observe":
		return SampleRoot(ctx, plan.Root, time.Second)
	default:
		return SampleResult{}, ErrInvalid
	}
}
