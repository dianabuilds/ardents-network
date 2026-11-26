package session

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	endpointpublisher "github.com/dianabuilds/ardents-network/internal/endpoint/publisher"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// LocalHolderPlan binds the original local authority, role, and operation options.
type LocalHolderPlan struct {
	Root, Profile string
	Network       *state.AuthorityDeclaration
	Role          admission.AllocationRole
	Options       Config
}

// ExecutionHolderPlan binds one holder conversation to its original Execution grant.
type ExecutionHolderPlan struct {
	Holder                LocalHolderPlan
	Generation, Principal [32]byte
	Snapshot              string
}

// RunExecutionHolder owns the original Execution and holder lifetimes.
func RunExecutionHolder(ctx context.Context, config ExecutionHolderPlan, input io.ReadCloser, out, diagnostic io.Writer, live bool) (code int) {
	if ctx == nil || config.Holder.Network == nil || !funding.ValidAdmissionAuthority("", config.Holder.Network, config.Holder.Root) || config.Holder.Profile != "" {
		return 2
	}
	if live && config.Holder.Options.Route == nil {
		return 2
	}
	if config.Snapshot != "" && (!live || config.Holder.Role != admission.AllocationPublisher) {
		return 2
	}
	if config.Holder.Options.Publication != nil && (!live || config.Snapshot == "" || config.Holder.Role != admission.AllocationPublisher) {
		return 2
	}
	surface := execution.Connection
	if config.Holder.Role == admission.AllocationPublisher {
		surface = execution.Administration
	} else if config.Holder.Role != admission.AllocationUser {
		return 2
	}
	owner, err := executionruntime.New(execution.Config{ID: config.Generation, Grants: []execution.Grant{{Principal: config.Principal, Surface: surface}}})
	if err != nil {
		return 2
	}
	defer func() {
		if owner.Close() != nil {
			code = 1
		}
	}()
	if live {
		var invocation *executionruntime.Invocation
		if config.Snapshot != "" {
			invocation, err = endpointpublisher.LaunchSnapshotFile(ctx, owner, config.Principal, config.Snapshot)
		} else {
			invocation, err = owner.Launch(ctx, config.Principal, surface)
		}
		if err != nil {
			return 1
		}
		defer func() {
			if invocation.Close() != nil {
				code = 1
			}
		}()
		operation, err := invocation.BeginOperation(ctx)
		if err != nil {
			return 1
		}
		defer operation.Close()
		if operation.Check() != nil {
			return 1
		}
		// runHolderPlan joins every genuine Route/Stock/Network borrower before
		// completing the one operation and original worker cleanup.
		code = RunLocalHolder(operation.Context(), config.Holder.Root, config.Holder.Profile, config.Holder.Network, config.Holder.Role, config.Holder.Options, input, out, diagnostic, operation)
		operation.Close()
		if invocation.Close() != nil || !invocation.CompletedCurrent() || ctx.Err() != nil {
			return 1
		}
		return code
	}
	prepared, err := owner.Prepare(ctx, config.Principal, surface)
	if err != nil {
		return 1
	}
	defer func() {
		if prepared.Close() != nil {
			code = 1
		}
	}()
	if err := prepared.Check(); err != nil {
		return 1
	}
	return RunLocalHolder(prepared.Context(), config.Holder.Root, config.Holder.Profile, config.Holder.Network, config.Holder.Role, config.Holder.Options, input, out, diagnostic, prepared)
}
