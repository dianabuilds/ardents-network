package releasedecision

import "time"

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
	sourceRevision            string
	buildIdentity             string
	dependencyIdentity        string
	sbomIdentity              string
	attestationPolicy         string
	qualification             string
	protocolPhase             string
	buildSafetyNoNewWorkAfter time.Time
	buildSafetyTerminateAfter time.Time
}
