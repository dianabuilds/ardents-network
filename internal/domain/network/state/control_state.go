package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

func (s *State) loadDistributionState() error {
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
	state, err := distribution.Decode(raw)
	if err != nil || distribution.Digest(raw) != name {
		return errors.New("distribution generation is invalid")
	}
	if state.EpochFloor != 0 && (s.current == nil || state.EpochFloor > s.current.Snapshot.Epoch ||
		state.EpochFloor == s.current.Snapshot.Epoch && state.EpochDigest != s.current.Snapshot.Digest) {
		if err := s.recoverDistributionActive(state); err != nil {
			return err
		}
	}
	if s.current != nil && state.EpochFloor < s.current.Snapshot.Epoch {
		return errors.New("distribution security state is older than the active generation")
	}
	s.distribution = state
	return s.recoverPendingState()
}

func (s *State) recoverDistributionActive(state distribution.Record) error {
	name := fmt.Sprintf("%x", state.EpochDigest)
	decision, err := loadStoredChain(s.config, s.storage, name)
	if err != nil {
		return fmt.Errorf("recover distribution active generation: %w", err)
	}
	if decision.Header.Number != state.EpochFloor || decision.Header.Digest != state.EpochDigest {
		return errors.New("distribution active identity disagrees with its generation")
	}
	// The floor repair must not activate a retired-schema generation (F-50).
	if err := classifyRetainedClosedSchema(s.config, decision.Header, "recovered active"); err != nil {
		return err
	}
	if err := persistDecision(s.storage, &decision, true); err != nil {
		return fmt.Errorf("repair active generation pointer: %w", err)
	}
	s.current = &decision
	return nil
}

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

// retireStateLocked makes an unsafe publication or lost role guard terminal.
// Runtime callers hold s.mu; Open may call this before publishing the owner.
func (s *State) retireStateLocked() {
	s.closed = true
	s.invalidateRuntime()
	if s.workCancel != nil {
		s.workCancel()
	}
}
