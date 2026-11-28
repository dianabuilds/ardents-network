//go:build linux

package state

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	descriptorstore "github.com/dianabuilds/ardents-network/internal/domain/reachability/store"
	"github.com/dianabuilds/ardents-network/internal/platform/cgroup"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// RepairRetainedForwardingProducer abandons one stopped Forwarding duty after
// checking its original manager, empty cgroup and both retained leases.
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

// RepairRetainedJoinProducer abandons one stopped JOIN participant duty.
// Each participant retains its own root and fence.
func RepairRetainedJoinProducer(ctx context.Context, input Config, original *systemd.JoinRuntime, declaration string, writableRoots []string) error {
	native := joinRepairNative
	if original != nil {
		native.observe = func() (observation.Properties, observation.Properties, error) {
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

// RepairRetainedIntroductionProducer abandons one stopped Introduction duty.
// Slot claims and spend obligations remain outside this repair.
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

// RepairRetainedDescriptorProducer abandons one stopped Descriptor duty.
// While both retained leases are held, it validates and removes only an
// uncertain Store stage between the two original quiescence observations.
// The fifth writable root is the Descriptor Store.
func RepairRetainedDescriptorProducer(ctx context.Context, input Config, original *systemd.DescriptorRuntime, declaration string, writableRoots []string) error {
	native := descriptorRepairNative
	if original != nil {
		native.observe = func() (observation.Properties, observation.Properties, error) {
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
		return descriptorstore.RepairStage(descriptorstore.Config{Root: writableRoots[len(writableRoots)-1], Network: current.Snapshot.NetworkID})
	},
}
