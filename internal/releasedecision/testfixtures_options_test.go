package releasedecision

import "time"

var testRefTime = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

// syntheticOptions customises the produced repository. Each field
// corresponds to one authenticated target identity claim; tests
// override the field under test.
type syntheticOptions struct {
	rootVersion               int64
	expires                   time.Time
	targetPath                string
	artifactLength            int
	platform                  string
	architecture              string
	environment               string
	network                   string
	rootEnvironment           string
	rootNetwork               string
	sourceRevision            string
	buildIdentity             string
	dependencyIdentity        string
	sbomIdentity              string
	attestationPolicy         string
	qualification             string
	buildState                string
	protocolPhase             string
	buildSafetyNoNewWorkAfter time.Time
	buildSafetyTerminateAfter time.Time
	protocolOverlappedSince   time.Time
	omitProtocolOverlap       bool
	capacityNotReady          bool
	drainNotReady             bool
	emergencyReason           string
	emergencyExpiry           time.Time
	targetsSignatureCount     int
	omitSBOM                  bool
	unknownCustomField        bool
	attestationDigestMismatch bool
	attestationInputMismatch  bool
}
