package main

import (
	"context"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// Installed preparation retains the exact volatile holder while the owner
// obtains offline signed permission and drives genuine Route bootstrap.
// It is not accepting Endpoint startup or an authenticated Service Connection.
func runExecutionPreparation(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) (code int) {
	return runExecutionHolder(ctx, args, input, out, diagnostic, false)
}

func runExecutionRouteHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) int {
	return runExecutionHolder(ctx, args, input, out, diagnostic, true)
}

func runExecutionHolder(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer, live bool) (code int) {
	var config struct {
		Holder     holderPlan `json:"holder"`
		Generation [32]byte   `json:"generation"`
		Principal  [32]byte   `json:"principal"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil || config.Holder.Network == nil || !validAdmissionAuthority("", config.Holder.Network, config.Holder.Root) || config.Holder.Profile != "" {
		return 2
	}
	if live && config.Holder.Route == nil {
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
		invocation, err := owner.Launch(ctx, config.Principal, surface)
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
		code = runHolderPlan(operation.Context(), config.Holder, input, out, diagnostic, operation)
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
	return runHolderPlan(prepared.Context(), config.Holder, input, out, diagnostic, prepared)
}
