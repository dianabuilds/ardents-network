package release

import (
	"time"
)

// Inputs is one complete offline-import request. The package is byte-only:
// the caller supplies the trusted root, every metadata file referenced by
// the trusted set, the exact artifact bytes, and the local binding. The
// package does not assume any source identity, network, cache, repository
// administration, or signing interface.
type Inputs struct {
	// RootBytes is the initial trusted root.json. The caller owns its
	// provenance; the package compares its version and digest to the
	// durable floor.
	RootBytes []byte
	// Files carries every metadata file referenced by the trusted set plus
	// the consistent-snapshot variants. The key is the URL the candidate
	// fetcher would request, for example
	// "https://release.invalid/metadata/timestamp.json" and
	// "https://release.invalid/metadata/1.snapshot.json". Distributor
	// independence is achieved by letting two byte adapters populate Files
	// with identical bytes from different sources.
	Files map[string][]byte
	// TargetPath is the canonical target path inside top-level targets
	// (for example "ardents/windows-amd64/application").
	TargetPath string
	// Artifact is the exact artifact bytes the offline-import caller has
	// supplied. The package verifies length and digest against the target
	// identity returned by the trusted set.
	Artifact []byte
	// Local is the local platform/environment/network binding. The package
	// compares the candidate target's identity fields against Local.
	Local LocalEnvironment
}

// LocalEnvironment captures the exact local binding that release metadata
// must match. Missing or empty values for fields that should match the
// candidate are an explicit release-incompatible result.
type LocalEnvironment struct {
	// Environment is the local environment marker (development, h3-test, ...).
	Environment string
	// Network is the local network identity string the release is bound to.
	Network string
	// Platform is the exact OS family marker (for example "windows-amd64"
	// or "linux-amd64").
	Platform string
	// Architecture is the exact CPU architecture marker.
	Architecture string
	// RefTime is the fixed UTC reference time the evaluation captures for
	// every expiry check. The package never calls go-tuf's UnsafeSetRefTime
	// and never reads the wall clock after evaluation starts.
	RefTime time.Time
}

// FloorSet is the durable version + digest floor for the four top-level roles.
// A newly published root may exist before the three executable-metadata floors;
// once present, those three floors advance atomically and never decrease.
type FloorSet struct {
	// RootVersion is the active trusted root version.
	RootVersion int64
	// RootDigest is the SHA-256 of the active trusted root bytes.
	RootDigest []byte
	// TimestampVersion is the durable timestamp version.
	TimestampVersion int64
	// TimestampDigest is the SHA-256 of the durable timestamp bytes.
	TimestampDigest []byte
	// SnapshotVersion is the durable snapshot version.
	SnapshotVersion int64
	// SnapshotDigest is the SHA-256 of the durable snapshot bytes.
	SnapshotDigest []byte
	// TargetsVersion is the durable top-level targets version.
	TargetsVersion int64
	// TargetsDigest is the SHA-256 of the durable top-level targets bytes.
	TargetsDigest []byte
}

// targetIdentity is the authenticated, read-only identity promoted by Decision
// and reused by the metadata descriptor so identity fields have one owner.
type targetIdentity struct {
	Platform             string
	Architecture         string
	Environment          string
	Network              string
	ReleaseIdentity      string
	ReleaseVersion       int64
	SourceRevision       string
	BuildInputCommitment string
	BuildIdentity        string
	DependencyIdentity   string
	SBOMIdentity         string
	AttestationPolicy    string
	Qualification        string
	BuildState           string
	ProtocolPhase        string
}

// Decision is the bounded result of one Evaluate call. Floors is the
// successor floor set the package durably committed. A rejected executable
// may still expose a root published earlier in the required verification order.
type Decision struct {
	authorization *acceptedAuthorization
	// Outcome is the bounded runtime classification.
	Outcome Outcome
	// Path, Length, Digest capture an authenticated target identity when target
	// verification completed, including lifecycle outcomes that reject new work.
	Path   string
	Length int64
	Digest []byte
	// Identity fields are the explicit authenticated caller contract.
	Platform             string
	Architecture         string
	Environment          string
	Network              string
	ReleaseIdentity      string
	ReleaseVersion       int64
	SourceRevision       string
	BuildInputCommitment string
	BuildIdentity        string
	DependencyIdentity   string
	SBOMIdentity         string
	AttestationPolicy    string
	Qualification        string
	BuildState           string
	ProtocolPhase        string
	// BuildSafety classifies the build safety machine.
	BuildSafety Outcome
	// Protocol classifies the protocol machine.
	Protocol Outcome
	// ReferenceTime is the exact fixed local reference time, normalized to
	// UTC, captured by the evaluation for every expiry check.
	ReferenceTime time.Time
	// BuildSafetyNoNewWorkAfter is the authenticated descriptor value
	// captured after artifact, builder, and local binding checks passed.
	BuildSafetyNoNewWorkAfter time.Time
	// BuildSafetyTerminateAfter is the authenticated descriptor value
	// captured after artifact, builder, and local binding checks passed.
	BuildSafetyTerminateAfter time.Time
	// ProtocolTransitionDeadline is the authenticated emergency expiry
	// when the candidate carries an emergency transition, otherwise zero.
	ProtocolTransitionDeadline time.Time
	// RootVersion is the active trusted root version after the evaluation.
	RootVersion int64
	// Floors is the durable successor floor set the package committed.
	// On a rejected outcome Floors equals the previously stored value.
	Floors FloorSet
	// Notice is a short, stable reason string; it carries no secret.
	Notice string
	// CustodyNotice is always rendered with the decision. H3 threshold
	// identities and both rebuild records remain project-controlled.
	CustodyNotice string
}

// Authorization is an immutable, opaque proof that release verified one
// transaction-eligible decision. Its zero value is invalid. Callers can retain
// and pass it to update, but cannot construct or alter the verified decision
// it carries.
type Authorization struct {
	accepted *acceptedAuthorization
}

type acceptedAuthorization struct {
	decision Decision
}

// Authorization returns an opaque update authorization only for a decision
// that this package produced as release-accepted or no-update. Update requires
// release-accepted for a new activation and can use no-update only for its
// retained-predecessor rollback check. The authorization retains a private
// snapshot, so later mutation of the public Decision view cannot change what
// update receives.
func (decision Decision) Authorization() (Authorization, bool) {
	if decision.authorization == nil {
		return Authorization{}, false
	}
	return Authorization{accepted: decision.authorization}, true
}

// AcceptedDecision returns a defensive copy of the decision authenticated by
// release. It reports false for a zero or otherwise invalid authorization.
func (authorization Authorization) AcceptedDecision() (Decision, bool) {
	if authorization.accepted == nil {
		return Decision{}, false
	}
	return cloneDecision(authorization.accepted.decision), true
}

func authorize(decision Decision) Decision {
	decision.authorization = nil
	decision.authorization = &acceptedAuthorization{decision: cloneDecision(decision)}
	return decision
}

func cloneDecision(decision Decision) Decision {
	decision.Digest = append([]byte(nil), decision.Digest...)
	decision.Floors.RootDigest = append([]byte(nil), decision.Floors.RootDigest...)
	decision.Floors.TimestampDigest = append([]byte(nil), decision.Floors.TimestampDigest...)
	decision.Floors.SnapshotDigest = append([]byte(nil), decision.Floors.SnapshotDigest...)
	decision.Floors.TargetsDigest = append([]byte(nil), decision.Floors.TargetsDigest...)
	decision.authorization = nil
	return decision
}
