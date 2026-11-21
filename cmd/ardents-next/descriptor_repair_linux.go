//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/platform/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// runDescriptorRepair is the operator recovery action for a Descriptor
// invocation that died before its joined completion (recovery contract class G).
// It runs as the shared fixed Node service principal and clears only the
// unresolved node-duty participation fence after proving the original service is
// defunct. Under the same proofs it also clears an uncertain Store staging file
// left by a crash mid-Publish, validating every committed publication floor
// first. It initializes no root, resets no floor and starts no runtime; spend
// obligations and publication floors are never repaired. A subsequent retained
// start is a new independently checked invocation.
func runDescriptorRepair(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation := repairDescriptorInputs(bounded, args[0])
	if operation != nil {
		terminal := &sourceTerminal{firstOwner: "state", first: operation}
		if sourceFailureReport(diagnostic, "repair", terminal, nil) != nil {
			return 2
		}
		return 1
	}
	if _, err := io.WriteString(out, "repaired\n"); err != nil {
		return 2
	}
	return 0
}

func repairDescriptorInputs(ctx context.Context, path string) (operation error) {
	inputs, plan, err := acquireDescriptorRepairInputs(ctx, path)
	if err != nil {
		return err
	}
	var config state.Config
	defer func() {
		operation = errors.Join(operation, inputs.Close())
	}()
	network := (*networkAuthorityPlan)(plan.Network)
	config, err = networkStateConfig(network)
	if err != nil {
		return err
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	check := func() error { return inputs.Check() }
	config.PermitWork = check
	writableRoots := []string{network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot}
	manager, err := systemd.OpenDescriptorRuntime(ctx)
	if manager != nil {
		defer func() { operation = errors.Join(operation, manager.Close()) }()
	}
	if err != nil {
		return err
	}
	return state.RepairRetainedDescriptorProducer(ctx, config, manager, path, writableRoots)
}

// Recovery retains only the original root-controlled public declaration. It loads
// no transport private key and starts no observer goroutine or runtime.
func acquireDescriptorRepairInputs(ctx context.Context, path string) (_ sourceInputs, plan node.DescriptorPlan, result error) {
	if ctx == nil || ctx.Err() != nil {
		return sourceInputs{}, plan, errSourceInput
	}
	admission, deny := context.WithCancel(ctx)
	l := &sourceInputLifetime{ctx: admission, deny: deny, callback: make(chan struct{}), done: make(chan struct{})}
	inputs := sourceInputs{l}
	l.stop = context.AfterFunc(ctx, func() { l.refuse(ctx.Err()); close(l.callback) })
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	file, err := l.acquire(path, 16<<10, false)
	if err != nil {
		return sourceInputs{}, plan, err
	}
	native, ok := file.info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || file.info.Mode().Perm()&0022 != 0 {
		return sourceInputs{}, plan, errSourceInput
	}
	plan, err = node.DecodeDescriptorPlan(file.raw)
	if err != nil || plan.Network == nil || plan.ProcessProfile != processresource.DescriptorProfile {
		return sourceInputs{}, plan, errSourceInput
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.DescriptorRoot) {
		return sourceInputs{}, plan, errSourceInput
	}
	if err := inputs.Check(); err != nil {
		return sourceInputs{}, plan, err
	}
	return inputs, plan, nil
}
