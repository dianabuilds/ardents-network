package releasedecision

import (
	"context"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// Evaluate authenticates one offline-import release request and
// returns a bounded Decision. On release-accepted the package
// atomically publishes the successor floors through the supplied Store
// and returns the committed floors in Decision.Floors. On any other
// outcome the package returns the previously stored floors without
// committing any partial state.
//
// Evaluate is the only entry point of the package. It enforces the
// complete Stage 7 release-safety and protocol transition contract
// before any floor is published. It is safe to call concurrently with
// distinct Inputs and Store values; the package does not share
// in-memory state across calls.
func Evaluate(ctx context.Context, in Inputs, store Store) Decision {
	if store == nil {
		return reject(outcomeReleaseInvalid, "store is nil", nil)
	}
	if in.Local.RefTime.IsZero() {
		return reject(outcomeReleaseInvalid, "local reference time is missing", nil)
	}
	refTime := in.Local.RefTime.UTC()
	verified, decision := buildVerifiedSet(ctx, in, refTime)
	if decision.Outcome != "" {
		return decision
	}
	target, err := verified.updater.GetTargetInfo(in.TargetPath)
	if err != nil {
		return reject(outcomeReleaseInvalid, "target lookup failed", err)
	}
	descriptor, err := customIdentity(target)
	if err != nil {
		return reject(outcomeReleaseInvalid, "target identity is invalid", err)
	}
	targetDecision, err := verifyTargetIdentity(target, descriptor, in, in.Local)
	if err != nil {
		outcome := outcomeReleaseInvalid
		if errStringContains(err, "match the local binding") {
			outcome = outcomeReleaseIncompatible
		}
		return reject(outcome, "target identity check failed", err)
	}
	existing, err := store.ReadFloors()
	if err != nil {
		return reject(outcomeReleaseInvalid, "read existing floors failed", err)
	}
	rotation, err := checkRootRotation(verified.rootChain, existing)
	if err != nil {
		if rotation.conflict {
			return reject(outcomeReleaseConflict, err.Error(), err)
		}
		return reject(outcomeReleaseInvalid, err.Error(), err)
	}
	successor, err := successorFloors(verified, rotation)
	if err != nil {
		return reject(outcomeReleaseInvalid, err.Error(), err)
	}
	protocol := classifyProtocol(in.Local, refTime)
	build := classifyBuildSafety(in.Local, refTime)
	combined, notice := combineOutcomes(protocol, build)
	switch combined {
	case outcomeReleaseAccepted, outcomeNoUpdate:
		// Only publish floors when the candidate actually advances them.
		// NoUpdate leaves the existing durable floors untouched.
		if rotation.advanced || !equalFloorSet(successor, existing) {
			if err := store.CommitFloors(successor); err != nil {
				return reject(outcomeReleaseInvalid, "commit successor floors failed", err)
			}
		}
		targetDecision.Outcome = combined
		targetDecision.BuildSafety = build.classification
		targetDecision.Protocol = protocol.classification
		targetDecision.Floors = successor
		targetDecision.RootVersion = successor.RootVersion
		targetDecision.Notice = notice
		return targetDecision
	default:
		targetDecision.Outcome = combined
		targetDecision.BuildSafety = build.classification
		targetDecision.Protocol = protocol.classification
		targetDecision.Floors = existing
		targetDecision.Notice = formatProtocolError(combined, notice)
		return targetDecision
	}
}

// errStringContains reports whether the supplied error's message
// contains the supplied fragment. It is used to choose between
// release-invalid and release-incompatible.
func errStringContains(err error, fragment string) bool {
	if err == nil {
		return false
	}
	return containsString(err.Error(), fragment)
}

// containsString is a tiny inline helper that does not allocate.
func containsString(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// equalFloorSet reports whether two floor sets are byte-for-byte
// equal. The package uses it to decide whether the successor is
// materially new.
func equalFloorSet(a, b FloorSet) bool {
	return floorSetEqual(a, b)
}

// ensure compile-time reference to the metadata package so the
// dependency is not dropped on tidy.
var _ = metadata.TARGETS
