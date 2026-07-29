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
	"path/filepath"
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
