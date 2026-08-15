package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const maximumCellEvidence = 4 << 20

// RunCell executes and durably retains one qualification attempt.
func RunCell(ctx context.Context, input CellInput, adapter CellAdapter) (CellReceipt, error) {
	return runCell(ctx, input, adapter, time.Now)
}

func runCell(ctx context.Context, input CellInput, adapter CellAdapter,
	now func() time.Time) (receipt CellReceipt, returnErr error) {
	receipt = CellReceipt{Schema: "ardents-qualification-cell-receipt-v1", CellID: input.CellID,
		AttemptID: input.AttemptID, ManifestDigest: input.ManifestDigest, Candidate: candidateNotRun,
		Observation: observationInvalid, Cleanup: cleanupInvalid}
	if err := validateCellInput(input, adapter, now); err != nil {
		return receipt, err
	}

	var observationErr error
	if err := adapter.Prepare(ctx); err != nil {
		observationErr = fmt.Errorf("prepare qualification cell: %w", err)
	} else if err := adapter.Arm(ctx); err != nil {
		observationErr = fmt.Errorf("arm qualification cell observation: %w", err)
	} else {
		started, err := adapter.Release(ctx)
		if err != nil {
			observationErr = fmt.Errorf("release qualification workload: %w", err)
		} else if observation, err := adapter.Observe(ctx); err != nil {
			observationErr = fmt.Errorf("observe qualification cell: %w", err)
		} else if err := validateCellObservation(started, observation); err != nil {
			observationErr = err
		} else {
			receipt.ActiveNanos = observation.TerminalAt.Sub(started).Nanoseconds()
			if observation.Candidate != "" {
				receipt.Candidate, receipt.Reason = observation.Candidate, observation.Reason
			}
			frozen, err := adapter.Freeze(ctx)
			if err != nil {
				observationErr = fmt.Errorf("freeze qualification cell: %w", err)
			} else if err := validateFrozenCell(observation, frozen); err != nil {
				observationErr = err
			} else {
				receipt.Candidate, receipt.Reason = frozen.Candidate, frozen.Reason
				receipt.Observation = observationComplete
				receipt.Evidence = append(json.RawMessage(nil), frozen.Evidence...)
			}
		}
	}
	if observationErr != nil {
		if receipt.Reason == "" {
			receipt.Reason = observationErr.Error()
		} else {
			receipt.Reason = errors.Join(errors.New(receipt.Reason), observationErr).Error()
		}
	}

	cleanup, cleanupErr := adapter.Cleanup(context.WithoutCancel(ctx))
	if cleanupErr == nil && len(cleanup) > 0 && len(cleanup) <= maximumCellEvidence && json.Valid(cleanup) {
		receipt.Cleanup = cleanupComplete
		receipt.CleanupEvidence = append(json.RawMessage(nil), cleanup...)
	} else if cleanupErr == nil {
		cleanupErr = errors.New("cleanup qualification cell evidence is invalid")
	} else {
		cleanupErr = fmt.Errorf("cleanup qualification cell: %w", cleanupErr)
	}
	if cleanupErr != nil {
		receipt.Reason = errors.Join(observationErr, cleanupErr).Error()
	}
	if err := writeCellReceipt(input.ReceiptRoot, receipt); err != nil {
		return receipt, errors.Join(observationErr, cleanupErr, err)
	}
	return receipt, nil
}

func validateCellInput(input CellInput, adapter CellAdapter, now func() time.Time) error {
	if !safeIdentifier(input.CellID) || !safeIdentifier(input.AttemptID) ||
		!hexDigest(input.ManifestDigest) || input.ReceiptRoot == "" || adapter == nil || now == nil {
		return errors.New("qualification cell input is invalid")
	}
	return nil
}

func validateCellObservation(started time.Time, value CellObservation) error {
	if started.IsZero() || value.TerminalAt.IsZero() || !value.TerminalAt.After(started) ||
		value.Candidate != "" && value.Candidate != candidatePass && value.Candidate != candidateFail {
		return errors.New("qualification candidate observation is invalid")
	}
	return nil
}

func validateFrozenCell(observed CellObservation, value FrozenCell) error {
	if value.Candidate != candidatePass && value.Candidate != candidateFail ||
		observed.Candidate != "" && observed.Candidate != value.Candidate {
		return errors.New("frozen candidate result is invalid")
	}
	if len(value.Evidence) == 0 || len(value.Evidence) > maximumCellEvidence || !json.Valid(value.Evidence) {
		return errors.New("frozen cell evidence is invalid")
	}
	return nil
}
