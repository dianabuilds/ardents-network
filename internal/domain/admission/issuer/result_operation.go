package issuer

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// PreparedResult retains the exact detached issuance input until execution.
type PreparedResult struct {
	plan         Plan
	batch        []byte
	facts        admission.Facts
	kind         quota.Kind
	responseFile string
	issue        bool
}

// PrepareResult checks the complete declaration and opens the bounded batch
// before telemetry or any durable issuance effect.
func PrepareResult(ctx context.Context, raw []byte, issue bool) (PreparedResult, error) {
	declaration, err := issuance.DecodeIssueDeclaration(raw, issue)
	if err != nil {
		return PreparedResult{}, issuance.ErrInvalid
	}
	prepared := PreparedResult{
		plan: Plan{AdmissionRoot: declaration.Setup.AdmissionRoot, AdmissionBinding: declaration.Setup.AdmissionBinding,
			KeyRoot: declaration.Setup.KeyRoot, KeyBinding: declaration.Setup.KeyBinding, ResultRoot: declaration.Setup.ResultRoot},
		facts: declaration.Facts, kind: declaration.Kind, responseFile: declaration.ResponseFile, issue: issue,
	}
	if issue {
		if err := issuance.CheckExportDestination(ctx, declaration.ResponseFile); err != nil {
			return PreparedResult{}, err
		}
		prepared.batch, err = artifacts.ReadBoundedFile(declaration.BatchFile, 16<<10)
		if err != nil {
			return PreparedResult{}, errors.Join(issuance.ErrInvalid, err)
		}
	}
	return prepared, nil
}

// Execute joins issuance and any output publication before returning status.
// It clears the local batch regardless of the outcome.
func (prepared *PreparedResult) Execute(ctx context.Context) Completion {
	defer clear(prepared.batch)
	var result Result
	if prepared.issue {
		result = Issue(ctx, prepared.plan, prepared.batch, prepared.facts, prepared.kind)
	} else {
		result = Initialize(ctx, prepared.plan)
	}
	phase, outcome := result.Status()
	if result.Response != nil {
		phase = "export"
		if err := issuance.Export(ctx, prepared.responseFile, result.Response); err != nil {
			outcome = ErrorOutcome(err)
		}
	}
	return Completion{Phase: phase, Outcome: outcome}
}
