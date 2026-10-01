package state

import (
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
)

// acceptedClosedEpochSchema is the sole AREP envelope schema for new closed
// Route Epoch candidates (F-50, ADR-0111). The installed provisioning and
// canonical qualification fixtures already build AREP v3.
const acceptedClosedEpochSchema = byte(3)

// ErrLegacyEpochIntake reports a new closed Route candidate encoded in a
// retired AREP schema. The refusal happens before any commit, staging or
// activation. Retained predecessor chain members stay authenticated by the
// historical verifier, and a retained old current or pending generation is
// classified at Open as a *RecoveryRequiredError instead.
var ErrLegacyEpochIntake = errors.New("closed Epoch uses a retired AREP schema")

// requireClosedIntakeSchema gates every new-candidate closed intake:
// offline Accept and the Source-wave candidate branch. Both Source result
// forms pass this single choke point before a wave can count a decision
// valid; the exact current/pending reuse branches return retained decisions
// that Open classified and whose digest binds the schema byte.
func requireClosedIntakeSchema(config config, header epoch.Header) error {
	if config.acceptedProfile == closedRouteProfile && header.Version != acceptedClosedEpochSchema {
		return fmt.Errorf("%w: closed intake accepts only AREP v%d, got v%d",
			ErrLegacyEpochIntake, acceptedClosedEpochSchema, header.Version)
	}
	return nil
}

// classifyRetainedClosedSchema refuses a recovered current, repaired active
// or pending generation in a retired schema with the typed recovery outcome.
// The chain and control floors are preserved: no pointer is cleared and no
// lower generation is selected here.
func classifyRetainedClosedSchema(config config, header epoch.Header, role string) error {
	if config.acceptedProfile == closedRouteProfile && header.Version != acceptedClosedEpochSchema {
		return &RecoveryRequiredError{Reason: fmt.Sprintf(
			"%s generation uses retired AREP schema v%d; closed intake accepts only v%d",
			role, header.Version, acceptedClosedEpochSchema)}
	}
	return nil
}

var (
	errPersistentStateConflict = errors.New("network state has a persistent conflict")
	errPendingEpochConflict    = errors.New("candidate Epoch conflicts with the durable pending Epoch")
)

func (s *networkState) allowCandidateTransition(candidate epoch.Decision) error {
	if s.distribution.conflicting {
		return errPersistentStateConflict
	}
	if s.pendingDecision != nil && candidate.Header.Number == s.pendingDecision.Header.Number &&
		candidate.Header.Digest != s.pendingDecision.Header.Digest {
		return errPendingEpochConflict
	}
	return nil
}
