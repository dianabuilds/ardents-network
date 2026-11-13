package cgroup

import "context"

// CheckJoinQuiescent checks the fixed JOIN native scope only. An empty scope
// is not original domain completion, prior producer retirement or Start permission.
func CheckJoinQuiescent(ctx context.Context) error {
	return checkFixedQuiescent(ctx, "ardents-join.service")
}
