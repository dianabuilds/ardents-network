package release

import (
	"time"
)

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

type Decision struct {
	cause         error
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
	// EvidenceNotice is always rendered with the decision. H3 threshold
	// identities and both rebuild records remain project-controlled.
	EvidenceNotice string
}

type Authorization struct {
	accepted *acceptedAuthorization
}

type acceptedAuthorization struct {
	decision Decision
}

func (decision Decision) Authorization() (Authorization, bool) {
	if decision.authorization == nil {
		return Authorization{}, false
	}
	return Authorization{accepted: decision.authorization}, true
}

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

type Outcome string

const (
	OutcomeReleaseAccepted     Outcome = "release-accepted"
	OutcomeNoUpdate            Outcome = "no-update"
	OutcomeUpdateRequired      Outcome = "update-required"
	OutcomeReleaseExpired      Outcome = "release-expired"
	OutcomeReleaseConflict     Outcome = "release-conflict"
	OutcomeReleaseRevoked      Outcome = "release-revoked"
	OutcomeReleaseIncompatible Outcome = "release-incompatible"
	OutcomeReleaseUnavailable  Outcome = "release-unavailable"
	OutcomeReleaseInvalid      Outcome = "release-invalid"
)

const (
	outcomeReleaseAccepted     = OutcomeReleaseAccepted
	outcomeNoUpdate            = OutcomeNoUpdate
	outcomeUpdateRequired      = OutcomeUpdateRequired
	outcomeReleaseExpired      = OutcomeReleaseExpired
	outcomeReleaseConflict     = OutcomeReleaseConflict
	outcomeReleaseRevoked      = OutcomeReleaseRevoked
	outcomeReleaseIncompatible = OutcomeReleaseIncompatible
	outcomeReleaseUnavailable  = OutcomeReleaseUnavailable
	outcomeReleaseInvalid      = OutcomeReleaseInvalid
)

const (
	// maximumMetadataFileBytes caps one metadata object to 1 MiB.
	maximumMetadataFileBytes int64 = 1 << 20
	// maximumMetadataBytes caps the aggregate metadata of one evaluation to 8 MiB.
	maximumMetadataBytes int64 = 8 << 20
	// maximumArtifactBytes bounds the in-memory offline artifact supplied to
	// one decision. Maintained H3 executables are currently below 16 MiB.
	maximumArtifactBytes int64 = 64 << 20
	// maximumRoles caps the number of top-level role entries in one root.
	maximumRoles = 32
	// maximumKeys caps the number of keys in one root.
	maximumKeys = 64
	// maximumSignatures caps the number of signatures on one role metadata.
	maximumSignatures = 64
	// maximumTargets caps the number of target descriptions in one Targets.
	maximumTargets = 512
	// maximumFetches caps the number of fetches per evaluation.
	maximumFetches = 32
	// maximumRootRotations caps the number of consecutive root versions per
	// evaluation.
	maximumRootRotations int64 = 16
	// totalTopLevelKeys is the number of top-level release role keys. The
	// Stage 7 H3 test profile exercises the 3-of-5 ordinary and 4-of-5
	// emergency threshold mechanics on this exact count.
	totalTopLevelKeys = 5
	// ordinaryThreshold is the accepted ordinary protocol transition
	// threshold: 3-of-5 of the top-level release keys.
	ordinaryThreshold = 3
	// emergencyThreshold is the only threshold allowed to shorten protocol
	// overlap or bypass capacity readiness.
	emergencyThreshold = 4
	// protocolOverlapWindow is the minimum overlap period an ordinary
	// protocol generation must satisfy before it may become required.
	protocolOverlapWindow = 90 * 24 * time.Hour
	// maximumEmergencyDuration is the absolute upper bound on a 4-of-5
	// emergency transition's finite expiry. Ordinary metadata must ratify
	// or replace the emergency before it expires.
	maximumEmergencyDuration = 30 * 24 * time.Hour
	// floorFileSizeLimit caps each durable floor file in the owned state
	// root. The bound is generous because each floor file only contains
	// version plus digest; the test profile uses a stricter limit to fail
	// closed on accidental oversize commits.
	floorFileSizeLimit int64 = 4 * 1024
)
