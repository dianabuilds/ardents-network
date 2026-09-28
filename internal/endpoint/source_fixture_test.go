//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// fakeSourceFlight is the minimal FlightRef used by tests that plant a fake
// Source opening through the production Lifecycle API.
type fakeSourceFlight struct {
	cancelled bool
	joined    bool
}

func (flight *fakeSourceFlight) CancelFlight() { flight.cancelled = true }

func (flight *fakeSourceFlight) JoinFlight() { flight.joined = true }

// plantSourceHandle publishes one fake live Source opening on lifecycle
// through ReserveOpeningLocked/FinishOpeningLocked and returns its handle.
// The fake zero-valued route prefix serves identity and state assertions
// only; tests must never run network operations or force its retirement
// through it. The caller must hold the owning dutyContext mutex.
func plantSourceHandle(lifecycle *source.Lifecycle) *source.Handle {
	flight := &fakeSourceFlight{}
	if !lifecycle.ReserveOpeningLocked(flight) {
		panic("plantSourceHandle: Source lifecycle is already live or opening")
	}
	handle, ok := lifecycle.FinishOpeningLocked(flight, &client.ClosedSourcePrefix{}, func() {}, true)
	if !ok || handle == nil {
		panic("plantSourceHandle: Source publication refused")
	}
	return handle
}

// closeSourceHandle forces the finite Route lifetime of a REAL opened Source
// handle. It must not be used on planted fake prefixes.
func closeSourceHandle(handle *source.Handle) error {
	return source.TerminateRoute(handle)
}

// sourceRouteDone exposes the finite-lifetime signal of a real Source handle.
func sourceRouteDone(handle *source.Handle) <-chan struct{} {
	return source.RouteDone(handle)
}
