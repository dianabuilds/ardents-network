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
