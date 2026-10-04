package state

import (
	"errors"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

// selectEpochHistory adapts authenticated documents and committed history to
// the domain. It performs no verification, persistence or winner selection.
func (s *networkState) selectEpochHistory(candidates []epoch.Decision) (epoch.Decision, networkdomain.EpochSelection, error) {
	facts := func(decision *epoch.Decision) networkdomain.EpochFacts {
		if decision == nil {
			return networkdomain.EpochFacts{}
		}
		header := decision.Header
		return networkdomain.EpochFacts{Network: header.NetworkID, Number: header.Number, Digest: header.Digest,
			Previous: header.Previous, ValidFrom: header.ValidFrom, ValidUntil: header.ValidUntil}
	}
	history, err := networkdomain.RestoreEpochHistory(facts(s.current), facts(s.pendingDecision), s.distribution.conflicting)
	if err != nil {
		return epoch.Decision{}, networkdomain.EpochSelection{}, err
	}
	inputs := make([]networkdomain.EpochFacts, len(candidates))
	for index := range candidates {
		inputs[index] = facts(&candidates[index])
	}
	selection, err := history.Reconcile(inputs)
	if errors.Is(err, networkdomain.ErrConflictedHistory) {
		err = errPersistentStateConflict
	}
	if errors.Is(err, networkdomain.ErrPendingEpochConflict) {
		err = errPendingEpochConflict
	}
	if err != nil {
		return epoch.Decision{}, selection, err
	}
	return candidates[selection.Index()], selection, nil
}

func isEpochHistoryConflict(err error) bool {
	return errors.Is(err, networkdomain.ErrEpochConflict) || errors.Is(err, errPendingEpochConflict) || errors.Is(err, errPersistentStateConflict)
}

func (s *networkState) publishSelectedEpoch(selection networkdomain.EpochSelection, now time.Time, selected epoch.Decision, summary sourceWaveSummary) (Snapshot, error) {
	disposition, err := selection.DispositionAt(now)
	if err != nil {
		if commitErr := s.commitSourceFailure(now, summary.outcomes, summary.observedEpochs, summary.observedDigests); commitErr != nil {
			return Snapshot{}, commitErr
		}
		return Snapshot{}, errors.Join(errRefreshUnavailable, errors.New("selected Epoch expired before source wave completed"), err)
	}
	switch disposition {
	case networkdomain.EpochDeferred:
		return s.commitDeferredSourceWave(now, selected, summary)
	case networkdomain.EpochPending:
		return s.commitPendingSourceWave(now, selected, summary)
	default:
		return s.commitActiveSourceWave(now, selected, summary)
	}
}
