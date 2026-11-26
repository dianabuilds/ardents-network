//go:build linux

package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/reachability"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// RepairRetainedDescriptorProducer explicitly abandons a dead platform
// Descriptor's sole node-duty participation fence. It does not attest successful
// original completion. The original read-only manager connection, actual empty
// kernel scope and both exclusive retained leases are mandatory. No
// caller-supplied proof callback or boolean can authorize the removal. A new
// invocation rechecks normal admission. Spend obligations and publication floors
// are never repaired here.
//
// Descriptor shares the ardents-node account with sibling Node role units, so
// quiescence is per-unit cgroup only — never an account-wide process scan that
// would refuse legitimate siblings. writableRoots are the exact ordered platform
// write paths (network, participation, spend, hosting, descriptor); the fifth
// entry is the Store root.
//
// The Store lease is flock and dies with the process, so the Store root carries
// no durable fence to remove. A crash mid-Publish can instead leave the reserved
// uncertain staging name, which a retained opening refuses. After the first
// stopped-unit proof and while holding both retained leases, this adapter
// invokes the bounded reachability.RepairRetainedStoreStage to validate every
// committed floor and remove only that staging under the exclusive Store lease,
// then reobserves quiescence before the one participation commit. It never
// initializes a missing Store root, resets or evicts a floor, promotes staging
// or deletes foreign entries.
func RepairRetainedDescriptorProducer(ctx context.Context, input Config, original *systemd.DescriptorRuntime, declaration string, writableRoots []string) error {
	native := descriptorRepairNative
	if original != nil {
		native.observe = func() (systemd.Properties, systemd.Properties, error) {
			facts, err := original.Observe()
			return facts.Unit, facts.Service, err
		}
	}
	return repairRetainedNodeDutyProducer(ctx, input, native, declaration, writableRoots)
}

var descriptorRepairNative = nodeDutyRepairNative{
	role:      descriptorRepairRole,
	quiesce:   cgroup.CheckDescriptorRecoveryQuiescent,
	rootCount: 5,
	storeStage: func(current *epoch.Decision, writableRoots []string) error {
		return reachability.RepairRetainedStoreStage(reachability.StoreConfig{Root: writableRoots[len(writableRoots)-1], Network: current.Snapshot.NetworkID})
	},
}
