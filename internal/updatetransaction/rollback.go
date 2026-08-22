package updatetransaction

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// rollbackToPredecessor changes only the owned code selection after the
// retained decision has been bound to its immutable bytes. The failed
// candidate is removed only after the predecessor current record is durable.
func (store *ownedStore) rollbackToPredecessor(generation uint64, predecessor inspectedTuple) error {
	raw, err := encodeCurrent(currentSelection{Transaction: predecessor.Generation, Current: predecessor})
	if err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	if err := atomicReplaceCurrentNative(store.root, raw, deadline); err != nil {
		return err
	}
	current, err := readBoundedFile(filepath.Join(store.root, "current"), maximumRecordBytes)
	selection, decodeErr := decodeCurrent(current)
	if err != nil || decodeErr != nil || sha256.Sum256(current) == [32]byte{} || selection.Transaction != predecessor.Generation ||
		selection.Rollback != nil || selection.Current != predecessor {
		return errors.Join(errRecordInvalid, err, decodeErr)
	}
	directory := store.generationPath("generations", generation)
	for _, name := range []string{"artifact", "manifest.bin"} {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return err
		}
		if err := store.ops.syncDirectory(directory); err != nil {
			return err
		}
	}
	if err := os.Remove(directory); err != nil {
		return err
	}
	return store.ops.syncDirectory(filepath.Join(store.root, "generations"))
}

func rollbackIdentity(request Request, tuple inspectedTuple) CandidateIdentity {
	decision := request.rollbackDecision
	return CandidateIdentity{Generation: tuple.Generation, TargetPath: decision.Path, Length: decision.Length,
		Digest: tuple.Artifact, Platform: decision.Platform, Architecture: decision.Architecture,
		Environment: decision.Environment, Network: decision.Network}
}
