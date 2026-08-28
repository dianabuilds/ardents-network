package inspection

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// Config identifies the independently pinned alpha bundle and the three
// distinct inspection roots. None may point to an active Endpoint root.
type Config struct {
	Root       string
	Enrollment enrollment.Request
	At         time.Time
}

// Report is the bounded, non-authorizing H4-6A inspection projection.
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

// CorpusConfig combines a verified ACA1 bundle inspection with explicitly
// supplied ACA2/corpus bytes. Neither artifact contains a source location.
type CorpusConfig struct {
	Control Config
	Catalog []byte
	Corpus  []byte
}

// CorpusReport is a non-authorizing projection for a verified ACA2 Alpha Name
// Corpus. An Endpoint-owned floor must separately decide whether to retain it.
type CorpusReport struct {
	Control         Report
	Corpus          *alpha.Corpus
	CorpusAuthority ed25519.PublicKey
}

// Inspect validates one enrollment-pinned bundle, invokes every component's
// own verifier at the fixed time, and records only their dedicated inspection
// floors. It never executes the candidate artifact.
func Inspect(ctx context.Context, config Config) (Report, error) {
	return inspect(ctx, config)
}

// InspectCorpus verifies the enrolled ACA1 bundle and every fixed control
// component before checking the supplied ACA2 corpus under its independently
// enrollment-pinned authority. It never opens an Endpoint or a corpus floor.
func InspectCorpus(ctx context.Context, config CorpusConfig) (CorpusReport, error) {
	return inspectCorpus(ctx, config)
}
