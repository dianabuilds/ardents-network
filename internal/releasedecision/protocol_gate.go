package releasedecision

import (
	"errors"
	"fmt"
	"time"
)

// protocolState captures the classification of the protocol machine
// after the candidate's identity has been read. The state machine is
// independent of the build safety machine.
type protocolState struct {
	classification Outcome
	notice         string
}

// buildSafetyState captures the classification of the build safety
// machine after the candidate's identity has been read.
type buildSafetyState struct {
	classification Outcome
	notice         string
}

// classifyProtocol evaluates the protocol machine. The 90-day ordinary
// gate, the capacity readiness gate, and the bounded 4-of-5 emergency
// transition are all applied to the local environment and the
// candidate's identity.
func classifyProtocol(local LocalEnvironment, refTime time.Time) protocolState {
	// An emergency expiry without a named reason is malformed: a
	// credible safety reason is a precondition of the bounded
	// transition, not a documentation nicety.
	if !local.EmergencyExpiry.IsZero() && local.EmergencyReason == "" {
		return protocolState{classification: outcomeReleaseInvalid, notice: "emergency expiry is set without a named safety reason"}
	}
	if local.EmergencyReason != "" {
		if local.EmergencyExpiry.IsZero() {
			return protocolState{classification: outcomeReleaseInvalid, notice: "emergency reason is set without a finite expiry"}
		}
		if refTime.After(local.EmergencyExpiry) {
			return protocolState{classification: outcomeReleaseUnavailable, notice: "emergency transition has expired; the affected capability is unavailable"}
		}
		if refTime.Add(maximumEmergencyDuration).Before(local.EmergencyExpiry) {
			return protocolState{classification: outcomeReleaseInvalid, notice: "emergency transition exceeds the bounded duration"}
		}
		return protocolState{classification: outcomeReleaseAccepted, notice: "emergency transition is within the bounded window"}
	}
	if !local.CapacityReady {
		return protocolState{classification: outcomeUpdateRequired, notice: "current-generation capacity is not yet qualified"}
	}
	if !local.DrainReady {
		return protocolState{classification: outcomeUpdateRequired, notice: "drain reserve is not yet qualified"}
	}
	if local.ProtocolOverlappedSince.IsZero() {
		return protocolState{classification: outcomeNoUpdate, notice: "no overlapping protocol generation is known"}
	}
	if refTime.Sub(local.ProtocolOverlappedSince) < protocolOverlapWindow {
		return protocolState{classification: outcomeNoUpdate, notice: "ordinary protocol overlap window has not elapsed"}
	}
	return protocolState{classification: outcomeReleaseAccepted, notice: "ordinary protocol transition satisfies the overlap and capacity gates"}
}

// classifyBuildSafety evaluates the build safety machine. The decision
// is independent of the protocol machine; a safe superseded build may
// still be a valid rollback target.
func classifyBuildSafety(local LocalEnvironment, refTime time.Time) buildSafetyState {
	if local.BuildSafetyTerminateAfter.IsZero() || local.BuildSafetyNoNewWorkAfter.IsZero() {
		return buildSafetyState{classification: outcomeNoUpdate, notice: "build safety bounds are not yet bound to the local environment"}
	}
	if refTime.After(local.BuildSafetyTerminateAfter) {
		return buildSafetyState{classification: outcomeReleaseRevoked, notice: "build safety terminal bound has elapsed; recovery is new security work"}
	}
	if refTime.After(local.BuildSafetyNoNewWorkAfter) {
		return buildSafetyState{classification: outcomeUpdateRequired, notice: "build safety no-new-work bound has elapsed"}
	}
	return buildSafetyState{classification: outcomeReleaseAccepted, notice: "build safety bounds allow new work"}
}

// combineOutcomes picks the more restrictive outcome when two
// independent state machines disagree. The release decision is
// release-accepted only when every machine has accepted.
func combineOutcomes(protocol protocolState, build buildSafetyState) (Outcome, string) {
	if protocol.classification == outcomeReleaseInvalid {
		return protocol.classification, protocol.notice
	}
	if build.classification == outcomeReleaseInvalid {
		return build.classification, build.notice
	}
	if build.classification == outcomeReleaseRevoked {
		return build.classification, build.notice
	}
	if protocol.classification == outcomeReleaseUnavailable {
		return protocol.classification, protocol.notice
	}
	if build.classification == outcomeUpdateRequired && protocol.classification == outcomeReleaseAccepted {
		return build.classification, build.notice
	}
	if protocol.classification == outcomeUpdateRequired {
		return protocol.classification, protocol.notice
	}
	if build.classification == outcomeReleaseAccepted && protocol.classification == outcomeReleaseAccepted {
		return outcomeReleaseAccepted, "release is accepted by every state machine"
	}
	if protocol.classification == outcomeNoUpdate && build.classification == outcomeReleaseAccepted {
		return outcomeNoUpdate, protocol.notice
	}
	return outcomeReleaseInvalid, errors.New("state machines are in conflict").Error()
}

// formatProtocolError returns a stable Notice string for a combined
// state machine outcome. It is short, non-sensitive, and has no
// dynamic inputs.
func formatProtocolError(outcome Outcome, notice string) string {
	if notice == "" {
		return string(outcome)
	}
	return fmt.Sprintf("%s: %s", outcome, notice)
}
