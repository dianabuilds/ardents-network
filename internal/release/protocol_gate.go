package release

import (
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

type emergencyReason string

const (
	emergencyExploitableFlaw       emergencyReason = "credible-exploitable-flaw"
	emergencyCompromisedPrimitive  emergencyReason = "compromised-primitive-or-key"
	emergencySafetyIncompatibility emergencyReason = "demonstrated-safety-incompatibility"
)

// classifyProtocol evaluates the protocol machine. The 90-day ordinary
// gate, the capacity readiness gate, and the bounded 4-of-5 emergency
// transition are all applied to the local environment and the
// candidate's identity.
func classifyProtocol(policy targetIdentityDescriptor, refTime time.Time) protocolState {
	phase, ok := parseProtocolPhase(policy.ProtocolPhase)
	if !ok {
		return protocolState{classification: outcomeReleaseInvalid, notice: "protocol phase is unsupported"}
	}
	switch phase {
	case protocolAnnounced:
		if !policy.ProtocolOverlappedAt.IsZero() || policy.EmergencyReason != "" {
			return protocolState{classification: outcomeReleaseInvalid, notice: "announced protocol carries transition-only facts"}
		}
		return protocolState{classification: outcomeReleaseAccepted, notice: "announced protocol is not yet a required transition"}
	case protocolOverlapSupported, protocolPreferred:
		if policy.ProtocolOverlappedAt.IsZero() || policy.ProtocolOverlappedAt.After(refTime) || policy.EmergencyReason != "" {
			return protocolState{classification: outcomeReleaseInvalid, notice: "protocol overlap facts are invalid for the signed phase"}
		}
		return protocolState{classification: outcomeReleaseAccepted, notice: "protocol remains in its authenticated overlap phase"}
	case protocolRetired:
		return protocolState{classification: outcomeReleaseIncompatible, notice: "protocol generation is retired"}
	case protocolRequired:
		// The required transition is evaluated below.
	}
	// An emergency expiry without a named reason is malformed: a
	// credible safety reason is a precondition of the bounded
	// transition, not a documentation nicety.
	if !policy.EmergencyExpiry.IsZero() && policy.EmergencyReason == "" {
		return protocolState{classification: outcomeReleaseInvalid, notice: "emergency expiry is set without a named safety reason"}
	}
	if policy.EmergencyReason != "" {
		if policy.EmergencyExpiry.IsZero() {
			return protocolState{classification: outcomeReleaseInvalid, notice: "emergency reason is set without a finite expiry"}
		}
		if !validEmergencyReason(policy.EmergencyReason) {
			return protocolState{classification: outcomeReleaseInvalid, notice: "emergency transition has an unsupported safety reason"}
		}
		if refTime.After(policy.EmergencyExpiry) {
			return protocolState{classification: outcomeReleaseUnavailable, notice: "emergency transition has expired; the affected capability is unavailable"}
		}
		if refTime.Add(maximumEmergencyDuration).Before(policy.EmergencyExpiry) {
			return protocolState{classification: outcomeReleaseInvalid, notice: "emergency transition exceeds the bounded duration"}
		}
		return protocolState{classification: outcomeReleaseAccepted, notice: "emergency transition is within the bounded window"}
	}
	if policy.ProtocolOverlappedAt.IsZero() {
		return protocolState{classification: outcomeReleaseInvalid, notice: "required protocol has no authenticated overlap start"}
	}
	if policy.ProtocolOverlappedAt.After(refTime) {
		return protocolState{classification: outcomeReleaseInvalid, notice: "protocol overlap start is in the future"}
	}
	if !policy.CapacityReady {
		return protocolState{classification: outcomeUpdateRequired, notice: "current-generation capacity is not yet qualified"}
	}
	if !policy.DrainReady {
		return protocolState{classification: outcomeUpdateRequired, notice: "drain reserve is not yet qualified"}
	}
	if refTime.Sub(policy.ProtocolOverlappedAt) < protocolOverlapWindow {
		return protocolState{classification: outcomeNoUpdate, notice: "ordinary protocol overlap window has not elapsed"}
	}
	return protocolState{classification: outcomeReleaseAccepted, notice: "ordinary protocol transition satisfies the overlap and capacity gates"}
}

func validEmergencyReason(reason emergencyReason) bool {
	switch reason {
	case emergencyExploitableFlaw, emergencyCompromisedPrimitive, emergencySafetyIncompatibility:
		return true
	default:
		return false
	}
}

// classifyBuildSafety evaluates the build safety machine. The decision
// is independent of the protocol machine; a safe superseded build may
// still be a valid rollback target.
func classifyBuildSafety(policy targetIdentityDescriptor, refTime time.Time) buildSafetyState {
	qualification, ok := parseQualification(policy.Qualification)
	if !ok {
		return buildSafetyState{classification: outcomeReleaseInvalid, notice: "target qualification is unsupported"}
	}
	switch qualification {
	case qualificationRevoked:
		return buildSafetyState{classification: outcomeReleaseRevoked, notice: "target qualification is revoked"}
	case qualificationUnavailable:
		return buildSafetyState{classification: outcomeReleaseUnavailable, notice: "target qualification is unavailable"}
	case qualificationDevelopmentOnly:
		if policy.Environment != "development" {
			return buildSafetyState{classification: outcomeReleaseIncompatible, notice: "development-only target cannot enter this environment"}
		}
	case qualificationQualified:
		// Continue with the independent build lifecycle.
	}
	buildState, ok := parseBuildState(policy.BuildState)
	if !ok {
		return buildSafetyState{classification: outcomeReleaseInvalid, notice: "build state is unsupported"}
	}
	if buildState == buildRevoked {
		return buildSafetyState{classification: outcomeReleaseRevoked, notice: "build is revoked"}
	}
	if !refTime.Before(policy.BuildSafetyTermAfter) {
		return buildSafetyState{classification: outcomeReleaseRevoked, notice: "build safety terminal bound has elapsed; recovery is new security work"}
	}
	if !refTime.Before(policy.BuildSafetyNoNewAfter) {
		return buildSafetyState{classification: outcomeUpdateRequired, notice: "build safety no-new-work bound has elapsed"}
	}
	switch buildState {
	case buildCurrent:
		return buildSafetyState{classification: outcomeReleaseAccepted, notice: "current build safety bounds allow new work"}
	case buildSuperseded:
		return buildSafetyState{classification: outcomeReleaseAccepted, notice: "safe superseded build remains an authenticated rollback target"}
	case buildVulnerable:
		return buildSafetyState{classification: outcomeReleaseAccepted, notice: "vulnerable build remains within its exact signed safety bounds"}
	}
	return buildSafetyState{classification: outcomeReleaseInvalid, notice: "build state is unsupported"}
}

// combineOutcomes picks the more restrictive outcome when two
// independent state machines disagree. The release decision is
// release-accepted only when every machine has accepted.
func combineOutcomes(protocol protocolState, build buildSafetyState) (Outcome, string) {
	ordered := []Outcome{
		outcomeReleaseInvalid, outcomeReleaseRevoked, outcomeReleaseUnavailable,
		outcomeReleaseIncompatible, outcomeUpdateRequired, outcomeNoUpdate,
	}
	for _, outcome := range ordered {
		if protocol.classification == outcome {
			return outcome, protocol.notice
		}
		if build.classification == outcome {
			return outcome, build.notice
		}
	}
	if build.classification == outcomeReleaseAccepted && protocol.classification == outcomeReleaseAccepted {
		return outcomeReleaseAccepted, "release is accepted by every state machine"
	}
	return outcomeReleaseInvalid, "state machines are in conflict"
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
