//go:build linux

package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// RepairRetainedForwardingProducer explicitly abandons a dead platform
// Forwarding's sole node-duty participation fence. It does not attest successful
// original completion. The original read-only manager connection, actual empty
// kernel scope and both exclusive retained leases are mandatory. No caller-supplied
// proof callback or boolean can authorize the removal. A new invocation rechecks
// normal admission. Spend obligations are never repaired here.
//
// Forwarding shares the ardents-node account with sibling Node role units, so
// quiescence is per-unit cgroup only — never an account-wide process scan that
// would refuse legitimate siblings. writableRoots are the exact ordered platform
// write paths (network, participation, spend, hosting).
func RepairRetainedForwardingProducer(ctx context.Context, input Config, original *systemd.ForwardingRuntime, declaration string, writableRoots []string) error {
	native := forwardingRepairNative
	if original != nil {
		native.observe = func() (observation.Properties, observation.Properties, error) {
			facts, err := original.Observe()
			return facts.Unit, facts.Service, err
		}
	}
	return repairRetainedNodeDutyProducer(ctx, input, native, declaration, writableRoots)
}

var forwardingRepairNative = nodeDutyRepairNative{
	role:    forwardingRepairRole,
	quiesce: cgroup.CheckForwardingRecoveryQuiescent,
}
