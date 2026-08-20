package releasedecision

import "time"

// The bounds are the R-049 O1 maximum envelope. They apply to a single
// Evaluate call: per-metadata-file, aggregate, counts, and root rotations.
// Exceeding any bound is a bounded rejection without panic or partial state.
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
