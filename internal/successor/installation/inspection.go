// Read-only public installed inspection admission.
package installation

import (
	"context"
	"time"
)

// CheckResult is a read-only local integrity observation, never a fresh Release
// authorization, active invocation receipt or Service readiness.
type CheckResult struct {
	Status           string
	GenerationDigest string
	Role             string
}

// Check inspects the selected generation without opening Release history,
// repairing resources or starting a process. Native readers retain their
// original Installation lease through observation and physical cleanup.
func Check(ctx context.Context, root string) (CheckResult, error) {
	if ctx == nil {
		return CheckResult{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return CheckResult{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return checkInstalled(bounded, root)
}
