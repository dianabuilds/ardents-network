package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

const maximumSourceExposureHistory = 2

type distributionState struct {
	sequence            uint64
	epochFloor          uint64
	epochDigest         [32]byte
	trustedTimeFloor    int64
	conflicting         bool
	consecutiveFailures uint64
	backoffLevel        byte
	nextAutomatic       int64
	history             [][32]byte
	cycleID             uint64
	cycleActive         bool
	cyclePurpose        byte
	cycleStarted        int64
	cycleDeadline       int64
	attempts            [4]byte
	outcomes            [4]byte
	requestedDigests    [2][32]byte
	observedEpochs      [4]uint64
	observedDigests     [4][32]byte
	pendingDigest       [32]byte
	pendingValidFrom    int64
	cycleSeed           [32]byte
	sourceOrder         [2]byte
}

func (s *networkState) loadDistributionState() error {
	name, raw, err := s.storage.LoadControl()
	if err != nil {
		return fmt.Errorf("load distribution security state: %w", err)
	}
	if name == "" {
		if s.current != nil {
			return &RecoveryRequiredError{Reason: "distribution journal is missing from an active root"}
		}
		return nil
	}
	state, err := decodeDistributionState(raw)
	if err != nil || distributionDigest(raw) != name {
		return errors.New("distribution generation is invalid")
	}
	if state.epochFloor != 0 && (s.current == nil || state.epochFloor > s.current.Snapshot.Epoch ||
		state.epochFloor == s.current.Snapshot.Epoch && state.epochDigest != s.current.Snapshot.Digest) {
		if err := s.recoverDistributionActive(state); err != nil {
			return err
		}
	}
	if s.current != nil && state.epochFloor < s.current.Snapshot.Epoch {
		return errors.New("distribution security state is older than the active generation")
	}
	s.distribution = state
	return s.recoverPendingState()
}

func (s *networkState) recoverDistributionActive(state distributionState) error {
	name := fmt.Sprintf("%x", state.epochDigest)
	decision, err := loadStoredChain(s.config, s.storage, name)
	if err != nil {
		return fmt.Errorf("recover distribution active generation: %w", err)
	}
	if decision.Header.Number != state.epochFloor || decision.Header.Digest != state.epochDigest {
		return errors.New("distribution active identity disagrees with its generation")
	}
	// The floor repair must not activate a retired-schema generation (F-50).
	if err := classifyRetainedClosedSchema(s.config, decision.Header, "recovered active"); err != nil {
		return err
	}
	if err := persistDecision(s.storage, decision, true); err != nil {
		return fmt.Errorf("repair active generation pointer: %w", err)
	}
	s.current = &decision
	return nil
}

func (s *networkState) commitDistribution(state distributionState) error {
	raw := encodeDistributionState(state)
	name := distributionDigest(raw)
	if err := s.storage.CommitControl(name, raw); err != nil {
		return err
	}
	s.distribution = state
	return nil
}

func (s *networkState) commitActiveDecision(decision epoch.Decision, state distributionState) (result error) {
	if s.config.sourceInfo.Serving && s.current == nil {
		return errors.New("direct Source server has no current identity")
	}
	controlCommitted := false
	if s.config.sourceInfo.Serving {
		// Hold the role root across both publications. Other role owners cannot
		// observe a gap between the old and successor Source duties.
		roles, err := duty.OpenOperation(context.Background(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
		if err != nil {
			s.retireServingSourceLocked()
			return err
		}
		defer func() { result = errors.Join(result, roles.Close()) }()
		producer := sourceProducer("server", s.config.root)
		previous := sourceServerDuty(*s.current)
		if err := roles.Replace(producer, []duty.Duty{sourceServerDuty(decision)}); err != nil {
			protected, guardErr := roles.Conflict(previous.Identity, previous.Family)
			if guardErr != nil || !protected {
				s.retireServingSourceLocked()
				return errors.Join(err, guardErr, errors.New("previous serving Source duty is unavailable"))
			}
			return err
		}
		defer func() {
			if controlCommitted {
				return
			}
			if err := roles.Replace(producer, []duty.Duty{previous}); err != nil {
				// The old decision may still be served. Retire this owner if its
				// local collision guard could not be restored.
				s.retireServingSourceLocked()
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
	if err := s.commitDistribution(state); err != nil {
		return err
	}
	controlCommitted = true
	s.current = &decision
	if s.pendingDecision != nil && s.pendingDecision.Header.Digest == decision.Header.Digest {
		s.pendingDecision = nil
	}
	return persistDecision(s.storage, decision, true)
}

// retireServingSourceLocked makes a lost local collision guard terminal for
// this State owner. Callers hold s.mu; Close still joins the canceled work.
func (s *networkState) retireServingSourceLocked() {
	s.closed = true
	s.resourceProtect = true
	if s.workCancel != nil {
		s.workCancel()
	}
}
