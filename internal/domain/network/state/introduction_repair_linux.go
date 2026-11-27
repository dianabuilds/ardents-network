//go:build linux

package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// RepairRetainedIntroductionProducer explicitly abandons a dead platform
// Introduction's sole node-duty participation fence. It does not attest successful
// original completion. The original read-only manager connection, actual empty
// kernel scope and both exclusive retained leases are mandatory. No caller-supplied
// proof callback or boolean can authorize the removal. A new invocation rechecks
// normal admission. Slot claims and spend obligations are never repaired here.
//
// Unlike Source, Introduction shares the ardents-node account with sibling Node
// role units, so quiescence is per-unit cgroup only — never an account-wide process
// scan that would refuse legitimate siblings. writableRoots are the exact ordered
// platform write paths (network, participation, spend, hosting, introduction).
func RepairRetainedIntroductionProducer(ctx context.Context, input Config, original *systemd.IntroductionRuntime, declaration string, writableRoots []string) error {
	native := introductionRepairNative
	if original != nil {
		native.observe = func() (observation.Properties, observation.Properties, error) {
			facts, err := original.Observe()
			return facts.Unit, facts.Service, err
		}
	}
	return repairRetainedNodeDutyProducer(ctx, input, native, declaration, writableRoots)
}

var introductionRepairNative = nodeDutyRepairNative{
	role:    introductionRepairRole,
	quiesce: cgroup.CheckIntroductionRecoveryQuiescent,
}
