// Package deploymentjournal owns protected local persistence adapters for
// Deployment transactions. It does not own fencing state transitions,
// Authority truth, host mutation, or journal directory policy.
package deploymentjournal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"ardents/internal/deployment"
	"ardents/internal/storage"
)

// FenceFile persists one protected transaction with an atomic same-directory
// replacement. The caller owns directory protection and single-coordinator
// exclusion.
type FenceFile struct {
	Path string
}

// RejoinFile persists one protected Rejoin transaction with the same strict
// private-file and optimistic revision contract as FenceFile.
type RejoinFile struct {
	Path string
}

// RolloutFile persists one protected topology rollout transaction. Its
// companion OS lock makes each optimistic revision check and replacement one
// critical section across independent coordinator processes.
type RolloutFile struct {
	Path string
}

type rolloutFileLease struct {
	lock *storage.PrivateFileLock
}

func (lease *rolloutFileLease) Release() error {
	if lease == nil || lease.lock == nil {
		return nil
	}
	err := lease.lock.Close()
	lease.lock = nil
	if err != nil {
		return deployment.ErrRolloutJournalInvalid
	}
	return nil
}

func (store RolloutFile) AcquireOperation(
	_ context.Context,
) (deployment.RolloutJournalLease, error) {
	path := strings.TrimSpace(store.Path)
	if path == "" {
		return nil, deployment.ErrRolloutJournalInvalid
	}
	lock, err := storage.AcquirePrivateFileLock(path + ".operation.lock")
	if errors.Is(err, storage.ErrPrivateFileLockUnavailable) {
		return nil, deployment.ErrRolloutJournalConflict
	}
	if err != nil {
		return nil, deployment.ErrRolloutJournalInvalid
	}
	return &rolloutFileLease{lock: lock}, nil
}

func (store FenceFile) Load(_ context.Context) (deployment.FenceTransaction, bool, error) {
	path := strings.TrimSpace(store.Path)
	if path == "" {
		return deployment.FenceTransaction{}, false, deployment.ErrFenceJournalInvalid
	}
	raw, found, err := storage.ReadStrictPrivateFileBounded(
		path,
		deployment.MaxFenceJournalBytes,
	)
	if err != nil {
		return deployment.FenceTransaction{}, false, deployment.ErrFenceJournalInvalid
	}
	if !found {
		return deployment.FenceTransaction{}, false, nil
	}
	transaction, err := decodeFenceTransaction(raw)
	if err != nil {
		return deployment.FenceTransaction{}, false, err
	}
	return transaction, true, nil
}

func (store FenceFile) Save(
	ctx context.Context,
	expectedRevision uint64,
	transaction deployment.FenceTransaction,
) error {
	path := strings.TrimSpace(store.Path)
	if path == "" || transaction.Revision != expectedRevision+1 {
		return deployment.ErrFenceJournalInvalid
	}
	existing, found, err := store.Load(ctx)
	if err != nil {
		return err
	}
	if found {
		if existing.Revision != expectedRevision {
			return deployment.ErrFenceJournalConflict
		}
		if !deployment.SameFenceTransactionBinding(existing, transaction) {
			return deployment.ErrFenceJournalBinding
		}
		if !deployment.ValidFenceTransactionTransition(existing, transaction) {
			return deployment.ErrFenceJournalConflict
		}
	} else if expectedRevision != 0 || transaction.Phase != deployment.FencePhaseRequested {
		return deployment.ErrFenceJournalConflict
	}
	if err := deployment.ValidateFenceTransaction(transaction); err != nil {
		return err
	}
	raw, err := json.Marshal(transaction)
	if err != nil || len(raw) > deployment.MaxFenceJournalBytes {
		return deployment.ErrFenceJournalInvalid
	}
	if err := storage.ValidatePrivateDir(filepath.Dir(path)); err != nil {
		return deployment.ErrFenceJournalInvalid
	}
	if err := storage.AtomicWritePrivateFile(path, raw); err != nil {
		return deployment.ErrFenceJournalInvalid
	}
	return nil
}

func decodeFenceTransaction(raw []byte) (deployment.FenceTransaction, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var transaction deployment.FenceTransaction
	if err := decoder.Decode(&transaction); err != nil {
		return deployment.FenceTransaction{}, deployment.ErrFenceJournalInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return deployment.FenceTransaction{}, deployment.ErrFenceJournalInvalid
	}
	if err := deployment.ValidateFenceTransaction(transaction); err != nil {
		return deployment.FenceTransaction{}, err
	}
	return transaction, nil
}

func (store RejoinFile) Load(_ context.Context) (deployment.RejoinTransaction, bool, error) {
	path := strings.TrimSpace(store.Path)
	if path == "" {
		return deployment.RejoinTransaction{}, false, deployment.ErrRejoinJournalInvalid
	}
	raw, found, err := storage.ReadStrictPrivateFileBounded(
		path,
		deployment.MaxRejoinJournalBytes,
	)
	if err != nil {
		return deployment.RejoinTransaction{}, false, deployment.ErrRejoinJournalInvalid
	}
	if !found {
		return deployment.RejoinTransaction{}, false, nil
	}
	transaction, err := decodeRejoinTransaction(raw)
	if err != nil {
		return deployment.RejoinTransaction{}, false, err
	}
	return transaction, true, nil
}

func (store RejoinFile) Save(
	ctx context.Context,
	expectedRevision uint64,
	transaction deployment.RejoinTransaction,
) error {
	path := strings.TrimSpace(store.Path)
	if path == "" || transaction.Revision != expectedRevision+1 {
		return deployment.ErrRejoinJournalInvalid
	}
	existing, found, err := store.Load(ctx)
	if err != nil {
		return err
	}
	if found {
		if existing.Revision != expectedRevision {
			return deployment.ErrRejoinJournalConflict
		}
		if !deployment.SameRejoinTransactionBinding(existing, transaction) {
			return deployment.ErrRejoinJournalBinding
		}
		if !deployment.ValidRejoinTransactionTransition(existing, transaction) {
			return deployment.ErrRejoinJournalConflict
		}
	} else if expectedRevision != 0 || transaction.Phase != deployment.RejoinPhaseRequested {
		return deployment.ErrRejoinJournalConflict
	}
	if err := deployment.ValidateRejoinTransaction(transaction); err != nil {
		return err
	}
	raw, err := json.Marshal(transaction)
	if err != nil || len(raw) > deployment.MaxRejoinJournalBytes {
		return deployment.ErrRejoinJournalInvalid
	}
	if err := storage.ValidatePrivateDir(filepath.Dir(path)); err != nil {
		return deployment.ErrRejoinJournalInvalid
	}
	if err := storage.AtomicWritePrivateFile(path, raw); err != nil {
		return deployment.ErrRejoinJournalInvalid
	}
	return nil
}

func decodeRejoinTransaction(raw []byte) (deployment.RejoinTransaction, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var transaction deployment.RejoinTransaction
	if err := decoder.Decode(&transaction); err != nil {
		return deployment.RejoinTransaction{}, deployment.ErrRejoinJournalInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return deployment.RejoinTransaction{}, deployment.ErrRejoinJournalInvalid
	}
	if err := deployment.ValidateRejoinTransaction(transaction); err != nil {
		return deployment.RejoinTransaction{}, err
	}
	return transaction, nil
}

func (store RolloutFile) Load(
	_ context.Context,
) (deployment.RolloutTransaction, bool, error) {
	lock, err := store.acquireLock()
	if err != nil {
		return deployment.RolloutTransaction{}, false, err
	}
	defer lock.Close()
	return store.loadUnlocked()
}

func (store RolloutFile) acquireLock() (*storage.PrivateFileLock, error) {
	path := strings.TrimSpace(store.Path)
	if path == "" {
		return nil, deployment.ErrRolloutJournalInvalid
	}
	lock, err := storage.AcquirePrivateFileLock(path + ".revision.lock")
	if errors.Is(err, storage.ErrPrivateFileLockUnavailable) {
		return nil, deployment.ErrRolloutJournalConflict
	}
	if err != nil {
		return nil, deployment.ErrRolloutJournalInvalid
	}
	return lock, nil
}

func (store RolloutFile) loadUnlocked() (deployment.RolloutTransaction, bool, error) {
	path := strings.TrimSpace(store.Path)
	if path == "" {
		return deployment.RolloutTransaction{}, false, deployment.ErrRolloutJournalInvalid
	}
	raw, found, err := storage.ReadStrictPrivateFileBounded(
		path,
		deployment.MaxRolloutJournalBytes,
	)
	if err != nil {
		return deployment.RolloutTransaction{}, false, deployment.ErrRolloutJournalInvalid
	}
	if !found {
		return deployment.RolloutTransaction{}, false, nil
	}
	transaction, err := decodeRolloutTransaction(raw)
	if err != nil {
		return deployment.RolloutTransaction{}, false, err
	}
	return transaction, true, nil
}

func (store RolloutFile) Save(
	_ context.Context,
	expectedRevision uint64,
	transaction deployment.RolloutTransaction,
) error {
	lock, err := store.acquireLock()
	if err != nil {
		return err
	}
	defer lock.Close()
	path := strings.TrimSpace(store.Path)
	if path == "" || transaction.Revision != expectedRevision+1 {
		return deployment.ErrRolloutJournalInvalid
	}
	existing, found, err := store.loadUnlocked()
	if err != nil {
		return err
	}
	if found {
		if existing.Revision != expectedRevision {
			return deployment.ErrRolloutJournalConflict
		}
		if !deployment.SameRolloutTransactionBinding(existing, transaction) {
			return deployment.ErrRolloutJournalBinding
		}
		if !deployment.ValidRolloutTransactionTransition(existing, transaction) {
			return deployment.ErrRolloutJournalConflict
		}
	} else if expectedRevision != 0 ||
		transaction.Phase != deployment.RolloutPhasePreflighted {
		return deployment.ErrRolloutJournalConflict
	}
	if err := deployment.ValidateRolloutTransaction(transaction); err != nil {
		return err
	}
	raw, err := json.Marshal(transaction)
	if err != nil || len(raw) > deployment.MaxRolloutJournalBytes {
		return deployment.ErrRolloutJournalInvalid
	}
	if err := storage.ValidatePrivateDir(filepath.Dir(path)); err != nil {
		return deployment.ErrRolloutJournalInvalid
	}
	if err := storage.AtomicWritePrivateFile(path, raw); err != nil {
		return deployment.ErrRolloutJournalInvalid
	}
	return nil
}

func (store RolloutFile) Clear(
	_ context.Context,
	expected deployment.RolloutTransaction,
) error {
	lock, err := store.acquireLock()
	if err != nil {
		return err
	}
	defer lock.Close()
	path := strings.TrimSpace(store.Path)
	if path == "" ||
		(expected.Phase != deployment.RolloutPhaseCommitted &&
			expected.Phase != deployment.RolloutPhaseCompensated) {
		return deployment.ErrRolloutJournalInvalid
	}
	existing, found, err := store.loadUnlocked()
	if err != nil {
		return err
	}
	if !found || existing.Revision != expected.Revision {
		return deployment.ErrRolloutJournalConflict
	}
	if !deployment.SameRolloutTransactionBinding(existing, expected) {
		return deployment.ErrRolloutJournalBinding
	}
	if !reflect.DeepEqual(existing, expected) {
		return deployment.ErrRolloutJournalConflict
	}
	if err := os.Remove(path); err != nil {
		return deployment.ErrRolloutJournalInvalid
	}
	return nil
}

func decodeRolloutTransaction(
	raw []byte,
) (deployment.RolloutTransaction, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var transaction deployment.RolloutTransaction
	if err := decoder.Decode(&transaction); err != nil {
		return deployment.RolloutTransaction{}, deployment.ErrRolloutJournalInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return deployment.RolloutTransaction{}, deployment.ErrRolloutJournalInvalid
	}
	if err := deployment.ValidateRolloutTransaction(transaction); err != nil {
		return deployment.RolloutTransaction{}, err
	}
	return transaction, nil
}
