package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

// commitActiveDecision coordinates the serving Source duty, immutable Epoch,
// distribution floor, in-memory decision, and active pointer in that order.
func (s *networkState) commitActiveDecision(decision epoch.Decision, state distributionState) error {
	return s.commitActiveDecisionWithControl(decision, state, s.storage.CommitControl)
}

func (s *networkState) commitActiveDecisionWithControl(decision epoch.Decision, state distributionState, commit func(string, []byte) error) (result error) {
	if s.config.sourceInfo.Serving && s.current == nil {
		return errors.New("direct Source server has no current identity")
	}
	restorePreviousDuty := true
	if s.config.sourceInfo.Serving {
		// Hold the role root across both publications. Other role owners cannot
		// observe a gap between the old and successor Source duties.
		roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
		if err != nil {
			s.retireStateLocked()
			return err
		}
		defer func() { result = errors.Join(result, roles.Close()) }()
		producer := sourceProducer("server", s.config.root)
		previous := sourceServerDuty(*s.current)
		if err := roles.Replace(producer, []duty.Duty{sourceServerDuty(decision)}); err != nil {
			protected, guardErr := roles.Conflict(previous.Identity, previous.Family)
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
			if err := roles.Replace(producer, []duty.Duty{previous}); err != nil {
				// The old decision may still be served. Retire this owner if its
				// local collision guard could not be restored.
				s.retireStateLocked()
				result = errors.Join(result, fmt.Errorf("restore serving Source duty: %w", err))
			}
		}()
	}
	if err := persistDecision(s.storage, decision, false); err != nil {
		return err
	}
	state.epochFloor, state.epochDigest = decision.Header.Number, decision.Header.Digest
	if state.pendingDigest == decision.Header.Digest {
		state.pendingDigest, state.pendingValidFrom = [32]byte{}, 0
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
	s.current = &decision
	if s.pendingDecision != nil && s.pendingDecision.Header.Digest == decision.Header.Digest {
		s.pendingDecision = nil
	}
	return persistDecision(s.storage, decision, true)
}
