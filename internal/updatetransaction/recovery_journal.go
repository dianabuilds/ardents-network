package updatetransaction

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
)

// journalValidation is the immutable canonical, contiguous, commitment-
// bound chain produced by validateJournal. It records only facts the pure
// planner needs: parsed entries, computed predecessor commitments, observed
// elapsed monotonic order, and the verified first predecessor commitment.
type journalValidation struct {
	Generation        uint64
	Entries           []journalEntry
	RawEntries        [][]byte
	FirstPredecessor  [32]byte
	ChainIntact       bool
	MonotonicIntact   bool
	EnvelopeIntact    bool
	PredecessorIntact bool
	Selected          bool
}

// errJournalInvalid reports every canonical, contiguous, commitment-bound
// journal failure. Recovery cannot proceed when this error is non-nil.
var errJournalInvalid = errors.New("update transaction journal is invalid")

// zeroDigest returns the all-zero SHA-256 used to reject obviously
// empty commitments without exposing the literal byte array.
func zeroDigest() [32]byte { return [32]byte{} }

// readJournalRaw reads the bounded journal directory as raw facts only.
// State-name mapping, decoding, and validation remain in this file and
// in validateJournal; the inventory reads bytes and never classifies a
// recovery row.
func readJournalRaw(root string) (journalRawEntries, error) {
	entries, err := recoveryReadDir(root, maximumJournalEntries)
	if err != nil {
		return nil, fmt.Errorf("%w: read journal: %v", errInventoryInvalid, err)
	}
	raws := journalRawEntries{}
	for _, entry := range entries {
		state, ok := journalStateFromName(entry.Name())
		if !ok {
			return nil, fmt.Errorf("%w: journal/%s not canonical", errInventoryInvalid, entry.Name())
		}
		raw, err := recoveryReadFile(filepath.Join(root, entry.Name()), maximumJournalBytes)
		if err != nil {
			return nil, fmt.Errorf("%w: journal/%s invalid", errInventoryInvalid, entry.Name())
		}
		raw.Name = entry.Name()
		raw.state = state
		raws[entry.Name()] = raw
	}
	return raws, nil
}

// journalStateFromName is the canonical state-code name lookup. The
// inventory and journal validator share this exact map; it lives here
// because journal decoding and state-name mapping remain solely in
// this file.
func journalStateFromName(name string) (byte, bool) {
	switch name {
	case "01-release-accepted.entry":
		return 1, true
	case "02-artifact-verified.entry":
		return 2, true
	case "03-staged.entry":
		return 3, true
	case "04-rollback-reserved.entry":
		return 4, true
	case "05-stop-new-work.entry":
		return 5, true
	case "06-draining.entry":
		return 6, true
	case "07-activated.entry":
		return 7, true
	case "08-self-testing.entry":
		return 8, true
	case "09-committed.entry":
		return 9, true
	case "10-rollback-pending.entry":
		return 10, true
	case "11-rolled-back.entry":
		return 11, true
	case "12-repair-required.entry":
		return 12, true
	}
	return 0, false
}

// validateJournal reconstructs the canonical contiguous chain for one
// canonical transaction. It walks state codes 1..9, stopping at the
// first missing entry to treat the observed prefix as the end of the
// chain; any subsequent entry file detected past the prefix is a
// structural gap and fails closed. The validator binds the canonical
// entry artifact/manifest commitments to the exact candidate physical
// facts admitted by the inventory, not merely to non-zero digests or
// a self-consistent attacker-selected chain.
func validateJournal(transaction uint64, raws journalRawEntries, predecessorCommitment, candidateArtifact, candidateManifest [32]byte) (journalValidation, error) {
	var validation journalValidation
	validation.Generation = transaction
	if transaction == 0 {
		return validation, fmt.Errorf("%w: transaction generation is zero", errJournalInvalid)
	}
	if len(raws) == 0 {
		return validation, fmt.Errorf("%w: no journal entries", errJournalInvalid)
	}
	predecessor := predecessorCommitment
	var elapsed uint64
	var hasMonotonic bool
	var transactionDeadline int64
	for state := transactionState(1); state <= stateRepairRequired; state++ {
		name, err := journalFileName(state)
		if err != nil {
			return validation, fmt.Errorf("%w: name %d: %v", errJournalInvalid, state, err)
		}
		raw, ok := raws[name]
		if !ok {
			if state == stateCommitted {
				pendingName, pendingErr := journalFileName(stateRollbackPending)
				pending, pendingOK := raws[pendingName]
				if pendingErr == nil && pendingOK && len(validation.Entries) > 0 &&
					validation.Entries[len(validation.Entries)-1].State == stateSelfTesting &&
					validation.Entries[len(validation.Entries)-1].AdapterResult == adapterFailed {
					state, name, raw, ok = stateRollbackPending, pendingName, pending, true
				}
			}
			if state == stateRolledBack {
				repairName, repairErr := journalFileName(stateRepairRequired)
				repair, repairOK := raws[repairName]
				if repairErr == nil && repairOK && len(validation.Entries) > 0 &&
					validation.Entries[len(validation.Entries)-1].State == stateRollbackPending {
					state, name, raw, ok = stateRepairRequired, repairName, repair, true
				}
			}
			if !ok {
				break
			}
		}
		entry, decodeErr := decodeJournalEntry(raw.Bytes)
		if decodeErr != nil {
			return validation, fmt.Errorf("%w: decode %s: %v", errJournalInvalid, name, decodeErr)
		}
		if entry.State != state {
			return validation, fmt.Errorf("%w: state mismatch in %s", errJournalInvalid, name)
		}
		if entry.Generation != transaction {
			return validation, fmt.Errorf("%w: generation mismatch in %s", errJournalInvalid, name)
		}
		if entry.ArtifactDigest == (zeroDigest()) || entry.ManifestCommitment == (zeroDigest()) {
			return validation, fmt.Errorf("%w: missing commitment in %s", errJournalInvalid, name)
		}
		if entry.Predecessor != predecessor {
			return validation, fmt.Errorf("%w: predecessor chain broken in %s", errJournalInvalid, name)
		}
		if entry.DeadlineUnix == 0 {
			return validation, fmt.Errorf("%w: zero deadline in %s", errJournalInvalid, name)
		}
		if !journalAdapterValid(entry.State, entry.AdapterResult) {
			return validation, fmt.Errorf("%w: adapter result invalid in %s", errJournalInvalid, name)
		}
		if entry.State == stateCommitted && entry.AdapterResult == adapterSuccess {
			if len(validation.Entries) == 0 || validation.Entries[len(validation.Entries)-1].State != stateSelfTesting ||
				validation.Entries[len(validation.Entries)-1].AdapterResult != adapterUnavailable {
				return validation, fmt.Errorf("%w: committed retry lacks unavailable self-test", errJournalInvalid)
			}
		}
		if entry.State == stateRollbackPending {
			if len(validation.Entries) == 0 || validation.Entries[len(validation.Entries)-1].State != stateSelfTesting ||
				validation.Entries[len(validation.Entries)-1].AdapterResult != adapterFailed {
				return validation, fmt.Errorf("%w: rollback pending lacks failed self-test", errJournalInvalid)
			}
		}
		if entry.State == stateRepairRequired {
			if len(validation.Entries) == 0 || validation.Entries[len(validation.Entries)-1].State != stateRollbackPending {
				return validation, fmt.Errorf("%w: repair-required lacks rollback pending", errJournalInvalid)
			}
		}
		if entry.State == stateRolledBack {
			if len(validation.Entries) == 0 || validation.Entries[len(validation.Entries)-1].State != stateRollbackPending {
				return validation, fmt.Errorf("%w: rolled-back lacks rollback pending", errJournalInvalid)
			}
		}
		if state == stateReleaseAccepted {
			transactionDeadline = entry.DeadlineUnix
		} else if state == stateStopNewWork {
			if entry.DeadlineUnix > transactionDeadline {
				return validation, fmt.Errorf("%w: stop deadline extended in %s", errJournalInvalid, name)
			}
		} else if entry.DeadlineUnix != transactionDeadline {
			return validation, fmt.Errorf("%w: transaction deadline changed in %s", errJournalInvalid, name)
		}
		if hasMonotonic && entry.ElapsedNanos < elapsed {
			return validation, fmt.Errorf("%w: elapsed decreased in %s", errJournalInvalid, name)
		}
		validation.Entries = append(validation.Entries, entry)
		validation.RawEntries = append(validation.RawEntries, raw.Bytes)
		if state == stateReleaseAccepted {
			validation.FirstPredecessor = predecessor
		}
		predecessor = sha256.Sum256(raw.Bytes)
		elapsed = entry.ElapsedNanos
		hasMonotonic = true
	}
	lastState := transactionState(0)
	if len(validation.Entries) > 0 {
		lastState = validation.Entries[len(validation.Entries)-1].State
	}
	for state := lastState + 1; state <= stateRepairRequired; state++ {
		name, _ := journalFileName(state)
		if raw, ok := raws[name]; ok && raw.state != 0 {
			return validation, fmt.Errorf("%w: gap in chain %s present without lower states", errJournalInvalid, name)
		}
	}
	if candidateArtifact != (zeroDigest()) && candidateManifest != (zeroDigest()) {
		for _, entry := range validation.Entries {
			if entry.ArtifactDigest != candidateArtifact {
				return validation, fmt.Errorf("%w: artifact mismatch in %s", errJournalInvalid, journalNameFor(entry.State))
			}
			if entry.ManifestCommitment != candidateManifest {
				return validation, fmt.Errorf("%w: manifest mismatch in %s", errJournalInvalid, journalNameFor(entry.State))
			}
		}
	}
	validation.EnvelopeIntact = true
	validation.ChainIntact = true
	validation.MonotonicIntact = true
	validation.Selected = len(validation.Entries) > 0
	return validation, nil
}

func journalAdapterValid(state transactionState, result adapterResult) bool {
	switch state {
	case stateStopNewWork, stateDraining:
		return result == adapterSuccess || result == adapterFailed
	case stateSelfTesting:
		return result == adapterSuccess || result == adapterFailed || result == adapterUnavailable
	case stateActivated:
		return result == adapterNotCalled || result == adapterUnavailable
	case stateCommitted:
		return result == adapterNotCalled || result == adapterSuccess
	case stateRollbackPending:
		return result == adapterNotCalled || result == adapterFailed
	case stateRolledBack:
		return result == adapterSuccess
	case stateRepairRequired:
		return result == adapterNotCalled || result == adapterFailed
	default:
		return result == adapterNotCalled
	}
}

func journalNameFor(state transactionState) string {
	name, _ := journalFileName(state)
	return name
}

// journalFirstPredecessorConfirmed is exposed for the planner; it asserts
// the canonical predecessor envelope digest equals the chain's first
// predecessor so a post-replacement R10/R11 trace can verify its
// predecessor commitment independently.
func journalFirstPredecessorConfirmed(validation journalValidation, expected [32]byte) error {
	if validation.FirstPredecessor != expected {
		return fmt.Errorf("%w: predecessor envelope mismatch", errJournalInvalid)
	}
	return nil
}
