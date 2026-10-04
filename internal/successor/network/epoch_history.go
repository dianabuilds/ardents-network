package network

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	ErrEpochConflict        = errors.New("conflicting Epoch identities")
	ErrPendingEpochConflict = fmt.Errorf("pending Epoch: %w", ErrEpochConflict)
	ErrConflictedHistory    = errors.New("network history has a persistent conflict")
	ErrEpochSequence        = errors.New("epoch does not continue accepted history")
	ErrEpochExpired         = errors.New("epoch is no longer current")
)

// EpochFacts are supplied by the authentication/persistence boundary. They
// contain no signature proof and must never be used as an authorization token.
type EpochFacts struct {
	Network, Digest, Previous [32]byte
	Number                    uint64
	ValidFrom, ValidUntil     time.Time
}

func (epoch EpochFacts) valid() bool {
	return epoch.Network != [32]byte{} && epoch.Digest != [32]byte{} && epoch.Number != 0 &&
		!epoch.ValidFrom.IsZero() && epoch.ValidUntil.After(epoch.ValidFrom)
}

// EpochHistory is the immutable current/pending relation recovered from one
// committed history. Candidate evaluation cannot change a committed history.
type EpochHistory struct {
	current, pending EpochFacts
	conflicted       bool
}

func RestoreEpochHistory(current, pending EpochFacts, conflicted bool) (EpochHistory, error) {
	if current != (EpochFacts{}) && !current.valid() {
		return EpochHistory{}, ErrEpochSequence
	}
	if pending != (EpochFacts{}) && (!pending.valid() || !successor(current, pending)) {
		return EpochHistory{}, ErrEpochSequence
	}
	return EpochHistory{current: current, pending: pending, conflicted: conflicted}, nil
}

func successor(current, candidate EpochFacts) bool {
	return current.valid() && current.Number != math.MaxUint64 &&
		candidate.Network == current.Network && candidate.Number == current.Number+1 && candidate.Previous == current.Digest
}

// EpochSelection describes a proposed transition, not a committed acceptance.
// The owner must persist it before publishing new runtime facts.
type EpochSelection struct {
	epoch          EpochFacts
	index          int
	hasPredecessor bool
}

func (selection EpochSelection) Index() int { return selection.index }

type EpochDisposition uint8

const (
	EpochUnavailable EpochDisposition = iota
	EpochCurrent
	EpochPending
	EpochDeferred
)

// DispositionAt distinguishes retaining a pending successor from deferring a
// future genesis, which has no predecessor from which it could be recovered.
func (selection EpochSelection) DispositionAt(now time.Time) (EpochDisposition, error) {
	if !selection.epoch.valid() || now.IsZero() {
		return EpochUnavailable, ErrEpochSequence
	}
	if !now.Before(selection.epoch.ValidUntil) {
		return EpochUnavailable, ErrEpochExpired
	}
	if now.Before(selection.epoch.ValidFrom) {
		if !selection.hasPredecessor {
			return EpochDeferred, nil
		}
		return EpochPending, nil
	}
	return EpochCurrent, nil
}

// Reconcile considers every authenticated observation, including ones that
// will not win selection. Choosing the newest first must not hide a conflict.
func (history EpochHistory) Reconcile(candidates []EpochFacts) (EpochSelection, error) {
	if history.conflicted {
		return EpochSelection{}, ErrConflictedHistory
	}
	if len(candidates) == 0 {
		return EpochSelection{}, ErrEpochSequence
	}
	identities := make(map[uint64][32]byte, len(candidates)+2)
	for _, retained := range []EpochFacts{history.current, history.pending} {
		if retained.valid() {
			identities[retained.Number] = retained.Digest
		}
	}
	selected := 0
	for index, candidate := range candidates {
		if !candidate.valid() || history.current.valid() && candidate.Network != history.current.Network || candidate.Network != candidates[0].Network {
			return EpochSelection{}, ErrEpochSequence
		}
		if digest, exists := identities[candidate.Number]; exists && digest != candidate.Digest {
			if candidate.Number == history.pending.Number {
				return EpochSelection{}, ErrPendingEpochConflict
			}
			return EpochSelection{}, fmt.Errorf("%w at number %d", ErrEpochConflict, candidate.Number)
		}
		identities[candidate.Number] = candidate.Digest
		if candidate.Number > candidates[selected].Number {
			selected = index
		}
	}
	for _, candidate := range candidates {
		if candidate == history.current || candidate == history.pending {
			continue
		}
		if history.current == (EpochFacts{}) {
			if candidate.Number != 1 || candidate.Previous != [32]byte{} {
				return EpochSelection{}, ErrEpochSequence
			}
		} else if !successor(history.current, candidate) {
			return EpochSelection{}, ErrEpochSequence
		}
	}
	return EpochSelection{epoch: candidates[selected], index: selected, hasPredecessor: history.current.valid()}, nil
}
