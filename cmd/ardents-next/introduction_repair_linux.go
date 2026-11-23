//go:build linux

package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// runIntroductionRepair is the operator recovery action for an Introduction
// invocation that died before its joined completion (recovery contract class G).
// It runs as the shared fixed Node service principal and clears only the
// unresolved node-duty participation fence after proving the original service is
// defunct. It initializes no root, resets no floor and starts no runtime; slot
// claims and spend obligations are never repaired. A subsequent retained start is
// a new independently checked invocation.
func runIntroductionRepair(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation := repairIntroductionInputs(bounded, args[0])
	if operation != nil {
		terminal := &sourceTerminal{FirstOwner: "state", First: operation}
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

func repairIntroductionInputs(ctx context.Context, path string) (operation error) {
	inputs, plan, err := acquireIntroductionRepairInputs(ctx, path)
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
	writableRoots := []string{network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot}
	manager, err := systemd.OpenIntroductionRuntime(ctx)
	if manager != nil {
		defer func() { operation = errors.Join(operation, manager.Close()) }()
	}
	if err != nil {
		return err
	}
	return state.RepairRetainedIntroductionProducer(ctx, config, manager, path, writableRoots)
}

// Recovery retains only the original root-controlled public declaration. It loads
// no transport private key and starts no observer goroutine or runtime.
func acquireIntroductionRepairInputs(ctx context.Context, path string) (_ sourceInputs, plan node.IntroductionPlan, result error) {
	if ctx == nil || ctx.Err() != nil {
		return sourceInputs{}, plan, errSourceInput
	}
	inputs := artifacts.NewObservedInputs(ctx, nil)
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	file, err := inputs.Acquire(path, 16<<10, false)
	if err != nil {
		return sourceInputs{}, plan, err
	}
	native, ok := file.Info().Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || file.Info().Mode().Perm()&0022 != 0 {
		return sourceInputs{}, plan, errSourceInput
	}
	plan, err = node.DecodeIntroductionPlan(file.Bytes())
	if err != nil || plan.Network == nil || plan.ProcessProfile != processresource.IntroductionProfile {
		return sourceInputs{}, plan, errSourceInput
	}
	network := (*networkAuthorityPlan)(plan.Network)
	if !validAdmissionAuthority("", network, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) || !independentRouteRoots(network.Root, plan.ParticipationRoot, plan.SpendRoot, plan.HostingRoot, plan.IntroductionRoot) {
		return sourceInputs{}, plan, errSourceInput
	}
	if err := inputs.Check(); err != nil {
		return sourceInputs{}, plan, err
	}
	return inputs, plan, nil
}
