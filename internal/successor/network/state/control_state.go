package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

const maximumSourceExposureHistory = 2

// These values are persisted in the ARDS1D4 distribution journal. Renaming
// in-memory states must never renumber an existing byte.
// Zero is the unset purpose in a fresh root.
const sourceCyclePurposeRefresh byte = 1

const (
	sourceAttemptNotStarted byte = iota
	sourceAttemptInFlight
	sourceAttemptCompleted
	sourceAttemptFailed // includes a persisted interrupted attempt
)

const sourceDigestSlotOffset = 2

// LATEST uses slots 0 and 1; BY_DIGEST uses slots 2 and 3.
func digestAttemptSlot(sourceIndex int) int { return sourceDigestSlotOffset + sourceIndex }

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
	return s.commitDistributionWithControl(state, s.storage.CommitControl)
}

func (s *networkState) commitDistributionWithControl(state distributionState, commit func(string, []byte) error) error {
	if s.closed {
		return errors.New("network state is closed")
	}
	raw := encodeDistributionState(state)
	name := distributionDigest(raw)
	if err := commit(name, raw); err != nil {
		if errors.Is(err, durable.ErrPointerSyncUncertain) {
			s.terminalErr = fmt.Errorf("distribution control commit is uncertain: %w", err)
			s.retireStateLocked()
		} else if state.conflicting && !s.distribution.conflicting {
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
func (s *networkState) retireStateLocked() {
	s.closed = true
	if s.workCancel != nil {
		s.workCancel()
	}
}
