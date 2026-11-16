//go:build linux

package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// runSourceRepair is the operator recovery action for a Source invocation that
// died before its joined completion (recovery contract class G). It runs as the
// fixed Source service principal (the same input custody as serve-retained) and
// clears only the unresolved serving producer after proving the original service
// is defunct. It creates no history, resets no floor and starts no runtime; a
// subsequent serve-retained is a new independently checked invocation.
func runSourceRepair(ctx context.Context, args []string, out, diagnostic io.Writer) (code int) {
	if ctx == nil || ctx.Err() != nil || len(args) != 1 {
		return 2
	}
	inputs, plan, err := acquireSourceInputs(ctx, args[0])
	if err != nil {
		return sourceStartupFailure(diagnostic, "input", err, nil)
	}
	var operation error
	var config state.Config
	defer func() {
		if key, ok := config.Source.ServeCertificate.PrivateKey.(ed25519.PrivateKey); ok {
			clear(key)
		}
		inputErr := inputs.Close()
		if operation != nil || inputErr != nil {
			code = 1
			terminal := &sourceTerminal{firstOwner: "state", first: operation, inputCleanup: inputErr}
			if sourceFailureReport(diagnostic, "repair", terminal, nil) != nil {
				code = 2
			}
		}
	}()
	config, err = networkStateConfig(plan.Network)
	if err != nil {
		operation = err
		return 2
	}
	config.LocalRoleStateRoot = plan.ParticipationRoot
	check := func() error { return inputs.Check() }
	config.PermitWork = check
	if err = sourceServingInputs(&config, plan, inputs, check); err != nil {
		operation = &sourceTerminal{firstOwner: "input", first: err}
		return 1
	}
	// Process-death evidence: the original platform service must be inactive with
	// no live main or control process. Lease release, elapsed time and exit status
	// are not completion evidence; the repair itself re-verifies durable consistency.
	verifyInactive := func(ctx context.Context) error {
		unit, service, err := systemd.Infrastructure(ctx, "source")
		if err != nil {
			return err
		}
		if !systemd.Matches(unit, "ActiveState", "s", "inactive") ||
			!systemd.Matches(service, "MainPID", "u", uint32(0)) ||
			!systemd.Matches(service, "ControlPID", "u", uint32(0)) {
			return errors.New("original Source service is not defunct")
		}
		return nil
	}
	operation = state.RepairRetainedSourceProducer(config, verifyInactive)
	if operation != nil {
		return 1
	}
	if _, err := io.WriteString(out, "repaired\n"); err != nil {
		operation = err
		return 1
	}
	return 0
}
