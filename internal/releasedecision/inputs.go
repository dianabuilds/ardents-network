package releasedecision

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
	// ProtocolOverlappedSince is the moment the current protocol generation
	// entered overlap-supported. An ordinary transition to required needs at
	// least 90 days since this moment.
	ProtocolOverlappedSince time.Time
	// CapacityReady is true when every Role Domain plus the required
	// control/discovery role has qualified current-generation capacity.
	CapacityReady bool
	// DrainReady is true when the bounded drain reserve is qualified for
	// every required role.
	DrainReady bool
	// EmergencyExpiry is the moment an expiring 4-of-5 emergency
	// transition must be ratified into ordinary metadata. The zero value
	// means no active emergency.
	EmergencyExpiry time.Time
	// EmergencyReason names the accepted safety reason for the emergency
	// transition: a credible exploitable flaw, a compromised primitive or
	// key, or a demonstrated safety incompatibility. Empty means no
	// active emergency.
	EmergencyReason string
	// BuildSafetyNoNewWorkAfter bounds the build safety state. Beyond this
	// moment a superseded build may not accept new network work.
	BuildSafetyNoNewWorkAfter time.Time
	// BuildSafetyTerminateAfter is the terminal bound on the same state.
	// Beyond this moment recovery requires new security work.
	BuildSafetyTerminateAfter time.Time
}

// FloorSet is the durable version + digest floor for the four top-level
// release roles. The same-version/different-digest or lower-version inputs
// from the candidate are release-invalid; the package never lowers the
// floor. FloorSet is the security watermark; the go-tuf cache is
// disposable and never participates in the watermark.
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

// Decision is the bounded result of one Evaluate call. Floors is the
// successor floor set the package durably committed; a non-accepted
// outcome leaves Floors unchanged on disk and FloorState reflects the
// previously published value. Notice carries a short human-readable
// reason string the caller may include in logs; it carries no secret and
// is bounded to a short fixed list.
type Decision struct {
	// Outcome is the bounded runtime classification.
	Outcome Outcome
	// Path, Length, Digest capture the accepted target identity for a
	// release-accepted or no-update outcome; they are zero otherwise.
	Path   string
	Length int64
	Digest []byte
	// Identity captures the full authenticated target identity the
	// candidate declared; the caller can render and log it directly.
	Platform           string
	Architecture       string
	Environment        string
	Network            string
	SourceRevision     string
	BuildIdentity      string
	DependencyIdentity string
	SBOMIdentity       string
	AttestationPolicy  string
	Qualification      string
	ProtocolPhase      string
	// BuildSafety classifies the build safety machine.
	BuildSafety Outcome
	// Protocol classifies the protocol machine.
	Protocol Outcome
	// RootVersion is the active trusted root version after the evaluation.
	RootVersion int64
	// Floors is the durable successor floor set the package committed.
	// On a rejected outcome Floors equals the previously stored value.
	Floors FloorSet
	// Notice is a short, stable reason string; it carries no secret.
	Notice string
}
