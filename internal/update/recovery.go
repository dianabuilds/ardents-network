package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
)

const (
	outcomeRecovered          = "recovered"
	outcomeCommitted          = "committed"
	outcomeResourceDenied     = "resource-denied"
	outcomeCleanupIncomplete  = "cleanup-incomplete"
	outcomeTransactionInvalid = "transaction-invalid"
)

const (
	stateBusyInvalidPublic = "transaction-invalid"
	stateBusyPublic        = "busy"
)

const (
	noticeUpdateInterrupted        = "update interrupted"
	noticeUpdateCommitted          = "update committed"
	noticeUpdateTransactionBusy    = "update transaction busy"
	noticeUpdateCleanupIncomplete  = "update cleanup incomplete"
	noticeUpdateTransactionInvalid = "update transaction invalid"
)

// recoverWithOperations runs one bounded recovery pass against the
// supplied root using the provided typed operations. Public Recover
// always supplies native operations; only the cleanup fault/delay
// tests may inject custom remove, move, replace, and sync functions
// for the per-invocation private seam. The seam cannot replace
// locking, inventory, decoding, validation, classification, clocks,
// deadlines, or Results. The public Interface is unchanged.
func recoverWithOperations(ctx context.Context, root string, ops cleanupOps) (Result, error) {
	if ctx == nil || ctx.Err() != nil {
		return invalidRecoverResult(0), errRecordInvalid
	}
	lock, err := acquireOwnedLock(root)
	if err != nil {
		if errors.Is(err, errLockBusy) {
			return busyRecoverResult(), err
		}
		return invalidRecoverResult(0), err
	}
	inventory, err := collectInventory(root)
	if err != nil {
		return invalidRecoverResult(0), errors.Join(err, releaseError(lock))
	}
	records, recordErr := reconstructRecoveryRecords(inventory)
	if recordErr != nil {
		return invalidRecoverResult(0), errors.Join(recordErr, releaseError(lock))
	}
	custody, custodyErr := recoveryCustodyFor(&inventory)
	if custodyErr != nil {
		return invalidRecoverResult(0), errors.Join(custodyErr, releaseError(lock))
	}
	var plan recoveryPlan
	var planErr error
	if inventory.InterruptedSelection == 0 {
		plan, planErr = planRecovery(inventory, journalValidation{}, records, custody)
	} else {
		raws := inventory.journalLookup(inventory.InterruptedSelection)
		candidateArtifact, candidateManifest := candidateCommitments(inventory, inventory.InterruptedSelection)
		journal, jErr := validateJournal(inventory.InterruptedSelection, raws, records.predecessorCommitment, candidateArtifact, candidateManifest)
		if jErr != nil {
			return invalidRecoverResult(0), errors.Join(jErr, releaseError(lock))
		}
		plan, planErr = planRecovery(inventory, journal, records, custody)
	}
	if planErr != nil {
		return invalidRecoverResult(0), errors.Join(planErr, releaseError(lock))
	}
	cleanupErr := executePlan(root, plan, ops)
	if cleanupErr != nil {
		return cleanupIncompleteResult(plan), errors.Join(cleanupErr, releaseError(lock))
	}
	if releaseErr := lock.release(); releaseErr != nil {
		return cleanupIncompleteResult(plan), fmt.Errorf("update transaction lock release: %w", releaseErr)
	}
	result := planToResult(plan)
	return result, recoveryTerminalError(result)
}

func recoveryTerminalError(result Result) error {
	switch result.Outcome {
	case "application-networking-unverified":
		return ErrSelfTestUnavailable
	case "rolled-back":
		return errRolledBack
	case "rollback-refused":
		return errRollbackRefused
	case "repair-required":
		return errRepairRequired
	}
	return nil
}

type recoveryRecords struct {
	predecessorCommitment [32]byte
	predecessorCurrent    []byte
}

// reconstructRecoveryRecords prepares canonical recovery bytes before the
// pure planner runs. It reuses exact observed predecessor-current bytes when
// they still exist; only a selected successor requires deterministic
// reconstruction from its physically verified rollback tuple.
func reconstructRecoveryRecords(facts inventoryResult) (recoveryRecords, error) {
	var records recoveryRecords
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return records, fmt.Errorf("reconstruct recovery records: %w", err)
	}
	predecessor := selection.Current
	if selection.Rollback == nil {
		records.predecessorCurrent = append([]byte(nil), facts.Current.Bytes...)
	} else {
		predecessor = *selection.Rollback
		records.predecessorCurrent, err = encodeCurrent(currentSelection{Transaction: predecessor.Generation, Current: predecessor})
		if err != nil {
			return records, fmt.Errorf("reconstruct predecessor current: %w", err)
		}
	}
	generation := generationByID(facts.Generations, predecessor.Generation)
	if generation == nil {
		return records, fmt.Errorf("%w: predecessor generation missing", errInventoryInvalid)
	}
	predecessorRaw, err := encodePredecessor(predecessorInspection{
		CurrentRecordDigest: sha256.Sum256(records.predecessorCurrent),
		Current:             predecessor,
		ArtifactObservation: sha256.Sum256(generation.Artifact.Bytes),
		ManifestObservation: sha256.Sum256(generation.Manifest.Bytes),
	})
	if err != nil {
		return records, fmt.Errorf("reconstruct predecessor evidence: %w", err)
	}
	records.predecessorCommitment = sha256.Sum256(predecessorRaw)
	return records, nil
}

func releaseError(lock *ownedLock) error {
	if err := lock.release(); err != nil {
		return fmt.Errorf("update transaction lock release: %w", err)
	}
	return nil
}

// Recover performs exactly one bounded restart-recovery pass for the
// existing transaction under root. Public Recover always uses
// production no-op/native private dependencies and never infers commit
// from file presence, executable success, or candidate-authored text.
func Recover(ctx context.Context, root string) (Result, error) {
	return recoverWithOperations(ctx, root, nativeCleanupOps())
}

// recoveryCustodyFor decodes the bounded current selection and returns
// the custody notice from the manifest of the selected generation.
func recoveryCustodyFor(facts *inventoryResult) (string, error) {
	if facts == nil || len(facts.Current.Bytes) == 0 {
		return "", errInventoryInvalid
	}
	selection, err := decodeCurrent(facts.Current.Bytes)
	if err != nil {
		return "", err
	}
	return custodyNoticeForTuple(*facts, selection.Current)
}

func candidateCommitments(facts inventoryResult, generation uint64) ([32]byte, [32]byte) {
	if candidate := generationByID(facts.StagingDirs, generation); candidate != nil {
		if !candidate.HasArtifact || !candidate.HasManifest {
			return [32]byte{}, [32]byte{}
		}
		return sha256.Sum256(candidate.Artifact.Bytes), sha256.Sum256(candidate.Manifest.Bytes)
	}
	if candidate := generationByID(facts.Generations, generation); candidate != nil {
		return sha256.Sum256(candidate.Artifact.Bytes), sha256.Sum256(candidate.Manifest.Bytes)
	}
	return [32]byte{}, [32]byte{}
}

// planToResult converts one deterministic pure plan into the public
// Result shape. Frozen notice vocabulary is reused without modification.
func planToResult(plan recoveryPlan) Result {
	return Result{
		Outcome:        plan.Outcome,
		State:          plan.State,
		Generation:     plan.Generation,
		CurrentDigest:  plan.CurrentDigest,
		RollbackDigest: plan.RollbackDigest,
		StagingPresent: plan.StagingPresent,
		SafeNotice:     plan.SafeNotice,
		CustodyNotice:  plan.CustodyNotice,
	}
}

// invalidRecoverResult returns the frozen transaction-invalid Result.
func invalidRecoverResult(generation uint64) Result {
	return Result{
		Outcome: outcomeTransactionInvalid, State: stateBusyInvalidPublic, Generation: generation,
		StagingPresent: false, SafeNotice: noticeUpdateTransactionInvalid,
	}
}

// busyRecoverResult returns the frozen resource-denied/busy Result.
func busyRecoverResult() Result {
	return Result{
		Outcome: outcomeResourceDenied, State: stateBusyPublic, Generation: 0,
		StagingPresent: false, SafeNotice: noticeUpdateTransactionBusy,
	}
}

// cleanupIncompleteResult returns the frozen cleanup-incomplete Result
// preserving the verified transaction generation and the last coherent
// state from the deterministic plan.
func cleanupIncompleteResult(plan recoveryPlan) Result {
	return Result{
		Outcome: outcomeCleanupIncomplete, State: plan.State, Generation: plan.Generation,
		StagingPresent: false, SafeNotice: noticeUpdateCleanupIncomplete,
	}
}
