package inspection

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

// Config identifies the independently pinned alpha bundle and the three
// distinct inspection roots. None may point to an active Endpoint root.
type Config struct {
	Root       string
	Enrollment enrollment.Request
	At         time.Time
}

// Report is the bounded, non-authorizing alpha-control inspection projection.
type Report struct {
	Inspection                  alphacontrol.Inspection
	CatalogCohort               string
	CatalogGeneration           uint64
	CatalogNotBefore            time.Time
	CatalogNotAfter             time.Time
	ComponentDetails            [3]ComponentDetails
	Release                     string
	ReleaseIdentity             string
	BuildIdentity               string
	ArtifactDigest              [32]byte
	ProtocolPhase               string
	BuildSafetyNoNewWorkAfter   time.Time
	BuildSafetyTerminateAfter   time.Time
	ReleaseAuthorizationPresent bool
	NetworkID                   [32]byte
	NetworkEpoch                uint64
	NetworkDigest               [32]byte
	NetworkProfile              string
	NetworkValidUntil           time.Time
}

// ComponentDetails is the exact verified catalog and statement identity for
// one fixed component. It remains diagnostic and grants no Endpoint authority.
type ComponentDetails struct {
	RootID              [32]byte
	Generation          uint64
	Digest              [32]byte
	NotBefore, NotAfter time.Time
}

// Inspect validates one enrollment-pinned bundle, invokes every component's
// own verifier at the fixed time, and records only their dedicated inspection
// floors. It never executes the candidate artifact.
func Inspect(ctx context.Context, config Config) (Report, error) {
	return inspect(ctx, config)
}
