//go:build linux

package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// RepairRetainedJoinProducer explicitly abandons a dead platform JOIN's sole
// node-duty participation fence. It does not attest successful original
// completion. The original read-only manager connection, actual empty kernel
// scope and both exclusive retained leases are mandatory. No caller-supplied
// proof callback or boolean can authorize the removal. A new invocation
// rechecks normal admission. Spend obligations are never repaired here.
//
// JOIN shares the ardents-node account with sibling Node role units, so
// quiescence is per-unit cgroup only — never an account-wide process scan that
// would refuse legitimate siblings. writableRoots are the exact ordered platform
// write paths (network, participation, spend, hosting). Each JOIN pair
// participant owns a separate participation root and fence; this repairs one
// invocation's sole fence, never the pair.
func RepairRetainedJoinProducer(ctx context.Context, input Config, original *systemd.JoinRuntime, declaration string, writableRoots []string) error {
	native := joinRepairNative
	if original != nil {
		native.observe = func() (systemd.Properties, systemd.Properties, error) {
			facts, err := original.Observe()
			return facts.Unit, facts.Service, err
		}
	}
	return repairRetainedNodeDutyProducer(ctx, input, native, declaration, writableRoots)
}

var joinRepairNative = nodeDutyRepairNative{
	role:    joinRepairRole,
	quiesce: cgroup.CheckJoinRecoveryQuiescent,
}
