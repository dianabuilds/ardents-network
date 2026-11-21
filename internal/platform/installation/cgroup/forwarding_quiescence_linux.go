package cgroup

import (
	"context"
	"os"
)

// CheckForwardingRecoveryQuiescent is the independent read-only kernel
// observation for explicit Forwarding participation-producer abandonment. It
// grants no original completion, manager, State or participation authority. Only
// the fixed service principal range may observe it; the repair separately binds
// the exact named account, loaded stopped service, original manager and the
// exclusive semantic leases.
//
// Forwarding shares the ardents-node account with the sibling Node role units, so
// an account-wide process exclusion would refuse legitimate siblings. Quiescence
// is therefore per-unit: the empty fixed Forwarding cgroup proves no process
// remains in this invocation's scope, while sibling units keep their own scopes.
// No /proc account scan is performed.
func CheckForwardingRecoveryQuiescent(ctx context.Context) error {
	if ctx == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return errInput
	}
	return observeFixedQuiescent(ctx, "ardents-forwarding.service")
}
