//go:build linux

package endpoint_test

import (
	"crypto/tls"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

// routeFixtureBindHost is the address the fixture route nodes listen on and
// advertise in the signed epoch. It defaults to loopback for single-host
// qualifications; a cross-host qualification sets it to the controller's
// reachable address so a participant on another host can dial the receivers.
var routeFixtureBindHost = "127.0.0.1"

func newRoleRouteFixtureConfigured(t *testing.T, profile transport.CarrierProfile, domain byte, introduction, issuerNetwork bool, configure func(*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate)) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	t.Helper()
	// Select a usable real permission hour before constructing any owner or
	// operation. Extending the signed profile does not extend an hourly token.
	// This mirrors the standalone Admission profile: wait, never retry or skip.
	if wait := time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)); wait < time.Minute {
		t.Log("waiting for a usable real permission hour before Route fixture creation")
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
	reservations := make(map[[32]byte]func())
	certificates := make(map[[32]byte]tls.Certificate)
	f := newNetworkAdmissionFixture(t, func(f *networkAdmissionFixture) {
		processfixture.AddRouteFixtureNodes(t, &f.spec, reservations, certificates, routeFixtureBindHost,
			string(profile), profile == transport.ClosedCarrierTCP, domain, introduction, issuerNetwork)
		if configure != nil {
			configure(f, reservations, certificates)
		}
	})
	return f, reservations, certificates
}

func publicationRouteNetwork(t *testing.T, carrier transport.CarrierProfile, issuing ...bool) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	return publicationRouteNetworkConfigured(t, carrier, nil, issuing...)
}

func publicationRouteNetworkConfigured(t *testing.T, carrier transport.CarrierProfile, configure func(*networkAdmissionFixture), issuing ...bool) (*networkAdmissionFixture, map[[32]byte]func(), map[[32]byte]tls.Certificate) {
	return newRoleRouteFixtureConfigured(t, carrier, 1, false, len(issuing) != 0 && issuing[0], func(f *networkAdmissionFixture, reservations map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) {
		processfixture.AddPublicationFixtureNodes(t, &f.spec, reservations, certificates, routeFixtureBindHost,
			string(carrier), carrier == transport.ClosedCarrierTCP, len(issuing) > 1 && issuing[1])
		if configure != nil {
			configure(f)
		}
	})
}
