package release

import (
	"context"
	"strings"
)

// Evaluate authenticates one offline-import release request against the
// verifier's exclusively owned durable state.
func (verifier *Verifier) Evaluate(ctx context.Context, in Inputs) Decision {
	if verifier == nil || verifier.store == nil {
		return reject(outcomeReleaseInvalid, "release verifier is nil", nil)
	}
	return evaluate(ctx, in, verifier.store)
}

// evaluate authenticates one offline-import release request and
// returns a bounded Decision. On release-accepted the package
// atomically publishes the successor floors through its private persistence
// and returns the committed floors in Decision.Floors. On any other
// outcome the package retains every root already durably verified in order,
// while executable metadata floors advance only for accepted/no-update data.
//
// evaluate is the private test seam. Verifier.Evaluate is the public entry
// point. It enforces the
// complete Stage 7 release-safety and protocol transition contract
// before executable metadata floors are published. It is safe to call concurrently with
// distinct Inputs and persistence values; the package does not share
// in-memory state across calls.
func evaluate(ctx context.Context, in Inputs, store floorPersistence) Decision {
	if store == nil {
		return reject(outcomeReleaseInvalid, "store is nil", nil)
	}
	if in.Local.RefTime.IsZero() {
		return reject(outcomeReleaseInvalid, "local reference time is missing", nil)
	}
	refTime := in.Local.RefTime.UTC()
	existing, err := store.ReadFloors()
	if err != nil {
		return reject(outcomeReleaseInvalid, "read existing floors failed", err)
	}
	startingFloors := existing
	verified, decision := buildVerifiedSet(ctx, in, refTime, store, startingFloors)
	if decision.Outcome != "" {
		return attachCurrentFloors(store, decision)
	}
	rejectCurrent := func(outcome Outcome, notice string, cause error) Decision {
		return attachCurrentFloors(store, reject(outcome, notice, cause))
	}
	target, ok := verified.set.Targets[targetRole].Signed.Targets[in.TargetPath]
	if !ok {
		return rejectCurrent(outcomeReleaseInvalid, "target lookup failed", nil)
	}
	descriptor, err := customIdentity(target)
	if err != nil {
		return rejectCurrent(outcomeReleaseInvalid, "target identity is invalid", err)
	}
	targetDecision, err := verifyTargetIdentity(target, descriptor, in, in.Local)
	if err != nil {
		outcome := outcomeReleaseInvalid
		if errStringContains(err, "match the local binding") {
			outcome = outcomeReleaseIncompatible
		}
		return rejectCurrent(outcome, "target identity check failed", err)
	}
	existing, err = store.ReadFloors()
	if err != nil {
		return rejectCurrent(outcomeReleaseInvalid, "read existing floors failed", err)
	}
	rotation, err := checkRootRotation(verified.rootChain, startingFloors)
	if err != nil {
		if rotation.conflict {
			return rejectCurrent(outcomeReleaseConflict, err.Error(), err)
		}
		return rejectCurrent(outcomeReleaseInvalid, err.Error(), err)
	}
	successor, err := successorFloors(verified, rotation)
	if err != nil {
		return rejectCurrent(outcomeReleaseInvalid, err.Error(), err)
	}
	if descriptor.EmergencyReason != "" {
		if err := verifyEmergencyThreshold(verified.set.Root, verified.targetsBytes); err != nil {
			return rejectCurrent(outcomeReleaseInvalid, "emergency threshold is not met", err)
		}
	}
	protocol := classifyProtocol(descriptor, refTime)
	build := classifyBuildSafety(descriptor, refTime)
	combined, notice := combineOutcomes(protocol, build)
	if combined == outcomeReleaseAccepted && existing.RootVersion != 0 && floorSetEqual(successor, existing) {
		combined = outcomeNoUpdate
		notice = "authenticated release metadata is unchanged"
	}
	switch combined {
	case outcomeReleaseAccepted, outcomeNoUpdate:
		// Only publish floors when the candidate actually advances them.
		// NoUpdate leaves the existing durable floors untouched.
		if rotation.advanced || !floorSetEqual(successor, existing) {
			if err := store.CommitFloors(successor, rootBytes(verified.rootChain)); err != nil {
				return rejectCurrent(outcomeReleaseInvalid, "commit successor floors failed", err)
			}
		}
		targetDecision.Outcome = combined
		targetDecision.BuildSafety = build.classification
		targetDecision.Protocol = protocol.classification
		targetDecision.Floors = successor
		targetDecision.RootVersion = successor.RootVersion
		targetDecision.Notice = notice
		targetDecision.CustodyNotice = h3CustodyNotice
		if combined == outcomeReleaseAccepted || combined == outcomeNoUpdate {
			targetDecision = authorize(targetDecision)
		}
		return targetDecision
	default:
		targetDecision.Outcome = combined
		targetDecision.BuildSafety = build.classification
		targetDecision.Protocol = protocol.classification
		targetDecision.Floors = existing
		targetDecision.Notice = formatProtocolError(combined, notice)
		targetDecision.CustodyNotice = h3CustodyNotice
		return targetDecision
	}
}

func attachCurrentFloors(store floorPersistence, decision Decision) Decision {
	floors, err := store.ReadFloors()
	if err == nil {
		decision.Floors = floors
		decision.RootVersion = floors.RootVersion
	}
	return decision
}

// errStringContains reports whether the supplied error's message
// contains the supplied fragment. It is used to choose between
// release-invalid and release-incompatible.
func errStringContains(err error, fragment string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), fragment)
}
