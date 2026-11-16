//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// runSourceRepair is the operator recovery action for a Source invocation that
// died before its joined completion (recovery contract class G). It runs as the
// fixed Source service principal (the same input custody as serve-retained) and
// clears only the unresolved serving producer after proving the original service
// is defunct. It initializes no root, resets no floor and starts no runtime; a
// subsequent serve-retained is a new independently checked invocation.
func runSourceRepair(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation := repairSourceInputs(bounded, args[0])
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

func repairSourceInputs(ctx context.Context, path string) (operation error) {
	inputs, plan, err := acquireSourceRepairInputs(ctx, path)
	if err != nil {
		return err
	}
	var config state.Config
	defer func() {
		operation = errors.Join(operation, inputs.Close())
	}()
	config, err = networkStateConfig(plan.Network)
	if err != nil {
		return err
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	check := func() error { return inputs.Check() }
	config.PermitWork = check
	manager, err := systemd.OpenSourceRuntime(ctx)
	if manager != nil {
		defer func() { operation = errors.Join(operation, manager.Close()) }()
	}
	if err != nil {
		return err
	}
	return state.RepairRetainedSourceProducer(ctx, config, manager, path)
}
