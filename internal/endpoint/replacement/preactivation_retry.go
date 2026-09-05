package replacement

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/release"
)

// replacementAuthorizationEligible admits a fresh Release authorization for a
// new replacement or for one retained preactivation retry. The caller still
// binds a no-update authorization to current executable and journal evidence.
func replacementAuthorizationEligible(decision release.Decision) bool {
	return (decision.Outcome == release.OutcomeReleaseAccepted || decision.Outcome == release.OutcomeNoUpdate) &&
		decision.BuildSafety == release.OutcomeReleaseAccepted && decision.Protocol == release.OutcomeReleaseAccepted
}

func (store *store) validateCompletedCandidate(current Record, programPath string) error {
	if _, err := store.prepared(); err == nil {
		return errors.New("endpoint replacement retains an unexpected prepared candidate")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	journalRecord, err := store.readJournal()
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Lstat(filepath.Join(store.root, rollbackName)); errors.Is(statErr, os.ErrNotExist) {
			return nil
		} else if statErr != nil {
			return statErr
		}
		return errors.New("endpoint replacement retains a predecessor without a completed journal")
	}
	if err != nil {
		return err
	}
	if (journalRecord.phase != "committed" && journalRecord.phase != "rollback-committed") ||
		journalRecord.candidate != current.Digest || !sameProgramPath(programPath, journalRecord.programPath) {
		return errors.New("endpoint replacement current candidate does not match its completed journal")
	}
	return store.ensureRetainedDigest(journalRecord.predecessor)
}

func (store *store) resumePreactivation(current, candidate Record, programPath string, predecessor []byte) (bool, error) {
	journalRecord, err := store.readJournal()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if journalRecord.phase == "committed" || journalRecord.phase == "rollback-committed" {
		return false, nil
	}
	if journalRecord.phase != "prepared" && journalRecord.phase != "rollback-retained" && journalRecord.phase != "staged" {
		return false, errors.New("endpoint replacement journal does not permit preactivation retry")
	}
	if !sameProgramPath(programPath, journalRecord.programPath) || journalRecord.predecessor != current.Digest || journalRecord.candidate != candidate.Digest {
		return false, errors.New("endpoint replacement journal does not match the current preactivation retry")
	}
	prepared, err := store.prepared()
	if err != nil || prepared != candidate {
		return false, errors.New("endpoint replacement prepared record does not match the preactivation retry")
	}
	if journalRecord.phase == "prepared" {
		if err := store.ensureRetainedPredecessor(current, predecessor); err != nil {
			return false, err
		}
		if err := store.writeJournal(journal{phase: "rollback-retained", programPath: journalRecord.programPath,
			predecessor: journalRecord.predecessor, candidate: journalRecord.candidate}); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := store.ensureRetainedDigest(journalRecord.predecessor); err != nil {
		return false, err
	}
	return true, nil
}

func (store *store) ensureRetainedPredecessor(current Record, predecessor []byte) error {
	retained, err := readProgram(filepath.Join(store.root, rollbackName))
	if errors.Is(err, os.ErrNotExist) {
		return writeExecutableAtomic(store.root, rollbackName, predecessor)
	}
	if err != nil {
		return err
	}
	digest := sha256.Sum256(retained)
	if int64(len(retained)) != current.Length || digest != current.Digest {
		return errors.New("endpoint replacement retained predecessor does not match the current record")
	}
	return nil
}

func (store *store) ensureRetainedDigest(expected [sha256.Size]byte) error {
	retained, err := readProgram(filepath.Join(store.root, rollbackName))
	if err != nil {
		return err
	}
	if digest := sha256.Sum256(retained); digest != expected {
		return errors.New("endpoint replacement retained predecessor does not match the journal")
	}
	return nil
}
