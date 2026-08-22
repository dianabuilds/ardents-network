package updatetransaction

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const rollbackRetireName = ".rollback-retire"

type rollbackRetirement struct{ PreviousCurrent []byte }

func encodeRollbackRetirement(previous []byte) ([]byte, error) {
	selection, err := decodeCurrent(previous)
	if err != nil || selection.Rollback == nil {
		return nil, errRecordInvalid
	}
	return encodeRecord(recordRollbackRetire, previous, maximumRecordBytes)
}

func decodeRollbackRetirement(raw []byte) (rollbackRetirement, error) {
	body, err := decodeRecord(raw, recordRollbackRetire, maximumRecordBytes)
	if err != nil {
		return rollbackRetirement{}, errRecordInvalid
	}
	selection, err := decodeCurrent(body)
	if err != nil || selection.Rollback == nil {
		return rollbackRetirement{}, errRecordInvalid
	}
	return rollbackRetirement{PreviousCurrent: append([]byte(nil), body...)}, nil
}

// retireRollback makes room for the next retained rollback payload. Its
// marker binds the old current record so Recover can complete only this exact
// bounded transition after a process interruption.
func (store *ownedStore) retireRollback(inspection rootInspection) error {
	if inspection.selection.Rollback == nil {
		return nil
	}
	currentPath := filepath.Join(store.root, "current")
	previous, err := readBoundedFile(currentPath, maximumRecordBytes)
	if err != nil || !bytes.Equal(previous, inspection.currentRaw) {
		return errors.Join(errRecordInvalid, err)
	}
	marker, err := encodeRollbackRetirement(previous)
	if err != nil {
		return err
	}
	markerPath := filepath.Join(store.root, rollbackRetireName)
	if err := writeNewFile(markerPath, marker); err != nil {
		return err
	}
	withoutRollback, err := encodeCurrent(currentSelection{Transaction: inspection.selection.Transaction, Current: inspection.selection.Current})
	if err != nil {
		return err
	}
	if err := atomicReplaceCurrentNative(store.root, withoutRollback, cleanupDeadlineAfter()); err != nil {
		return err
	}
	if err := removeGeneration(store.root, inspection.selection.Rollback.Generation); err != nil {
		return err
	}
	if err := removeTransaction(store.root, inspection.selection.Transaction); err != nil {
		return err
	}
	if err := os.Remove(markerPath); err != nil {
		return err
	}
	return store.ops.syncDirectory(store.root)
}

func retireForNextGeneration(store *ownedStore, inspection rootInspection, generation uint64) (*ownedStore, rootInspection, error) {
	if err := store.retireRollback(inspection); err != nil {
		return nil, rootInspection{}, errors.Join(errRecordInvalid, err, store.release())
	}
	if err := store.release(); err != nil {
		return nil, rootInspection{}, err
	}
	return acquireStore(store.root, generation)
}

func cleanupDeadlineAfter() time.Time { return time.Now().Add(5 * time.Second) }

func removeGeneration(root string, generation uint64) error {
	directory := filepath.Join(root, "generations", strconv.FormatUint(generation, 10))
	for _, name := range []string{"artifact", "manifest.bin"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return err
		}
	}
	if err := os.Remove(directory); err != nil {
		return err
	}
	return syncDirectoryNative(filepath.Join(root, "generations"))
}

func removeTransaction(root string, generation uint64) error {
	directory := filepath.Join(root, "transactions", strconv.FormatUint(generation, 10))
	journal := filepath.Join(directory, "journal")
	for state := stateReleaseAccepted; state <= stateRepairRequired; state++ {
		name, err := journalFileName(state)
		if err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(journal, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(journal); err != nil {
		return err
	}
	if err := os.Remove(directory); err != nil {
		return err
	}
	return syncDirectoryNative(filepath.Join(root, "transactions"))
}
