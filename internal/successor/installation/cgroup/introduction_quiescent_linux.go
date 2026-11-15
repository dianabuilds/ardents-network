package cgroup

import "context"

// CheckIntroductionQuiescent checks the fixed Introduction native scope only. An empty scope
// is not original domain completion, prior producer retirement or Start permission.
func CheckIntroductionQuiescent(ctx context.Context) error {
	return checkFixedQuiescent(ctx, "ardents-introduction.service")
}
