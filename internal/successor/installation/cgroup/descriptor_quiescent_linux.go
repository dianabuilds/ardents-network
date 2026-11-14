package cgroup

import "context"

// CheckDescriptorQuiescent checks the fixed Descriptor native scope only. An empty scope
// is not original domain completion, prior producer retirement or Start permission.
func CheckDescriptorQuiescent(ctx context.Context) error {
	return checkFixedQuiescent(ctx, "ardents-descriptor.service")
}
