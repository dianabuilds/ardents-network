package state

import (
	"context"
	"errors"
	"fmt"

	duty2 "github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// commitActiveDecision coordinates the serving Source duty, immutable Epoch,
// distribution floor, in-memory decision, and active pointer in that order.
func (s *State) commitActiveDecision(decision *epoch.Decision, state distribution.Record) error {
	return s.commitActiveDecisionWithControl(decision, state, s.storage.CommitControl)
}

func (s *State) commitActiveDecisionWithControl(decision *epoch.Decision, state distribution.Record, commit func(string, []byte) error) (result error) {
	if decision == nil {
		// Preserve the historical zero-value path: an unverified decision
		// reaches the same durable refusal without a nil dereference.
		decision = &epoch.Decision{}
	}
	if s.config.sourceInfo.Serving && s.current == nil {
		return errors.New("direct Source server has no current identity")
	}
	restorePreviousDuty := true
	var nextPredecessors []duty2.Duty
	if s.config.sourceInfo.Serving {
		// Hold the role root across both publications. Other role owners cannot
		// observe a gap between the old and successor Source duties.
		roles, err := duty2.OpenOperation(context.Background(), duty2.Config{Root: s.config.localRoles, Clock: s.config.clock})
		if err != nil {
			s.retireStateLocked()
			return err
		}
		defer func() { result = errors.Join(result, roles.Close()) }()
		producer := sourceProducer("server", s.config.root)
		previous := servingDutySet(s.current, s.servingPredecessors)
		if s.activeSource > 0 {
			nextPredecessors = previous
		}
		if err := roles.Replace(producer, servingDutySet(decision, nextPredecessors)); err != nil {
			old := sourceServerDuty(s.current)
			protected, guardErr := roles.Conflict(old.Identity, old.Family)
			if guardErr != nil || !protected {
				s.retireStateLocked()
				return errors.Join(err, guardErr, errors.New("previous serving Source duty is unavailable"))
			}
			return err
		}
		defer func() {
			if !restorePreviousDuty {
				return
			}
			if err := roles.Replace(producer, previous); err != nil {
				// The old decision may still be served. Retire this owner if its
				// local collision guard could not be restored.
				s.retireStateLocked()
				result = errors.Join(result, fmt.Errorf("restore serving Source duty: %w", err))
			}
		}()
	}
	if s.current == nil || s.current.Header.Digest != decision.Header.Digest {
		s.invalidateRuntime()
	}
	if err := persistDecision(s.storage, decision, false); err != nil {
		return err
	}
	state.EpochFloor, state.EpochDigest = decision.Header.Number, decision.Header.Digest
	if state.PendingDigest == decision.Header.Digest {
		state.PendingDigest, state.PendingValidFrom = [32]byte{}, 0
	}
	if err := s.commitDistributionWithControl(state, commit); err != nil {
		if errors.Is(err, durable.ErrPointerSyncUncertain) {
			// The successor may already be the durable floor. Keep its
			// collision guard while the retired owner refuses all work.
			restorePreviousDuty = false
		}
		return err
	}
	restorePreviousDuty = false
	// The decision is immutable after verification; callers never mutate it
	// after commit, so publishing the pointer keeps the same owned facts.
	s.current = decision
	if s.config.sourceInfo.Serving {
		s.servingPredecessors = nextPredecessors
	}
	if s.pendingDecision != nil && s.pendingDecision.Header.Digest == decision.Header.Digest {
		s.pendingDecision = nil
	}
	return persistDecision(s.storage, decision, true)
}

// commitPendingDecision writes the authenticated generation before the
// control journal can name it. History reconciliation precedes this operation.
func (s *State) commitPendingDecision(decision *epoch.Decision, next distribution.Record) error {
	if decision == nil {
		// Preserve the historical zero-value staging refusal.
		decision = &epoch.Decision{}
	}
	newPending := s.pendingDecision == nil
	if newPending {
		if err := stageGeneration(s.storage, decision); err != nil {
			return err
		}
	}
	next.PendingDigest, next.PendingValidFrom = decision.Header.Digest, decision.Header.ValidFrom.Unix()
	if err := s.commitDistribution(next); err != nil {
		return err
	}
	if newPending {
		// The staged decision is immutable; callers never mutate it after
		// commit, so the pointer keeps the same owned facts.
		s.pendingDecision = decision
	}
	return nil
}

// commitDistribution publishes a verified transition on the original State lease.
func (s *State) commitDistribution(state distribution.Record) error {
	return s.commitDistributionWithControl(state, s.storage.CommitControl)
}

func (s *State) commitDistributionWithControl(state distribution.Record, commit func(string, []byte) error) error {
	if s.closed {
		return errors.New("network state is closed")
	}
	raw := distribution.Encode(state)
	if state.Conflicting && !s.distribution.Conflicting {
		s.invalidateRuntime()
	}
	name := distribution.Digest(raw)
	if err := commit(name, raw); err != nil {
		if errors.Is(err, durable.ErrPointerSyncUncertain) {
			s.terminalErr = fmt.Errorf("distribution control commit is uncertain: %w", err)
			s.retireStateLocked()
		} else if state.Conflicting && !s.distribution.Conflicting {
			// The conflict has already been verified. An ordinary
			// pre-pointer failure cannot make the prior decision safe to serve.
			s.terminalErr = fmt.Errorf("persist verified network state conflict: %w", err)
			s.retireStateLocked()
		}
		return err
	}
	s.distribution = state
	return nil
}
