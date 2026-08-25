package inspection

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
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
	Inspection    alphacontrol.Inspection
	Release       string
	NetworkEpoch  uint64
	NetworkDigest [32]byte
}

// Inspect validates one enrollment-pinned bundle, invokes every component's
// own verifier at the fixed time, and records only their dedicated inspection
// floors. It never executes the candidate artifact.
func Inspect(ctx context.Context, config Config) (Report, error) {
	return inspect(ctx, config)
}
