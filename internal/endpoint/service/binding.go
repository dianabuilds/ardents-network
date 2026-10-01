//go:build linux

package service

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Binding is the Endpoint root's retained authority seam. The root's
// serviceBinding implements every method from the exact immutable job,
// publication, and duty-context facts; this package consumes the seam only
// while opening, authenticating, and retiring one protected stream.
type Binding interface {
	// Current revalidates the live job and unchanged permission authority.
	Current() error
	// Logical returns the immutable per-Connection logical context.
	Logical() [32]byte
	// Credential returns the independently verified publication Credential.
	Credential() servicepublication.Credential
	// Facts copies the immutable shared authority tuple.
	Facts() nativeconnection.ProtectedContextInput
	// Surface returns the local Application Interface role.
	Surface() broker.Surface
	// JobContext returns the bounded context of the exact bound job.
	JobContext() context.Context
	// WorkloadDirection returns the checked send/receive byte contract for
	// the local surface.
	WorkloadDirection() (uint32, uint32, error)
	// Clock reads the Endpoint generation clock.
	Clock() time.Time
	// Resources returns the Endpoint resource ledger.
	Resources() func(string, int) uint32
	// AcquirePublication leases the Publisher's current publication; the root
	// refuses it for a client-only Endpoint.
	AcquirePublication(ctx context.Context) (*servicepublication.Lease, error)
	// MatchesPublication reports the exact retained Credential and digest.
	MatchesPublication(current servicepublication.Current) bool
	// Recovery computes the immutable recovery authority request.
	Recovery() nativeconnection.Recovery
	// ValidateRecovery refuses any request that changed immutable authority.
	ValidateRecovery(request nativeconnection.Recovery) error
	// ReleaseIntroductionRecovery retires the binding's recovery slot.
	ReleaseIntroductionRecovery() error
}
