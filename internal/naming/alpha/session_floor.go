package alpha

import (
	"errors"
	"sync"
)

// SessionFloor retains the greatest observed signed corpus serial for one
// alpha cohort during one live resolution session. It deliberately makes no
// restart-survival claim; a durable corpus floor is a later product promotion
// gate rather than an implicit cache.
type SessionFloor struct {
	mu     sync.Mutex
	cohort string
	serial uint64
	digest [32]byte
}

// NewSessionFloor creates a floor for exactly one declared alpha cohort.
func NewSessionFloor(cohort string) (*SessionFloor, error) {
	if !validCohort(cohort) {
		return nil, errors.New("alpha corpus floor cohort is invalid")
	}
	return &SessionFloor{cohort: cohort}, nil
}

// Observe advances the session floor only to a later signed corpus. A prior
// serial is stale and a changed corpus with the same serial is a conflict.
func (floor *SessionFloor) Observe(corpus *Corpus) error {
	if floor == nil || corpus == nil || corpus.Cohort() != floor.cohort || corpus.Serial() == 0 {
		return errors.New("alpha corpus floor input is invalid")
	}
	digest := corpus.Digest()
	floor.mu.Lock()
	defer floor.mu.Unlock()
	if corpus.Serial() < floor.serial {
		return &ResolutionError{Failure: FailureStale}
	}
	if corpus.Serial() == floor.serial && floor.serial != 0 && digest != floor.digest {
		return &ResolutionError{Failure: FailureConflict}
	}
	if corpus.Serial() > floor.serial {
		floor.serial, floor.digest = corpus.Serial(), digest
	}
	return nil
}
