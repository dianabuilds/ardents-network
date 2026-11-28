//go:build linux && document_worker_installed

package endpoint_test

import (
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"

	"path/filepath"
	"testing"
	"time"

	descriptorstore "github.com/dianabuilds/ardents-network/internal/domain/reachability/store"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
)

func installedServiceNetwork(t *testing.T, carrier string, evidence ...*installedReceiverEvidence) (*networkAdmissionFixture, *descriptorstore.Store) {
	f, reservations, certificates := publicationRouteNetwork(t, transport.CarrierProfile(carrier), true)
	store, err := descriptorstore.Open(descriptorstore.Config{Root: t.TempDir(), Network: f.profile.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	var observed *installedReceiverEvidence
	if len(evidence) == 1 {
		observed = evidence[0]
	}
	installedPublisherReceiversObserved(t, f, reservations, certificates, store, observed, nil, nil)
	return f, store
}

// Approval stays with the fixture authority. A participant receives only the
// signed response to its actual qualified request, never this signing key.
func installedServiceApproval(t *testing.T, base, role string, request []byte, f *networkAdmissionFixture) []byte {
	t.Helper()
	return endpointfixture.ApproveFixturePermission(t, filepath.Join(base, "allocation-"+role), request, f.profile, f.spec.Authority, time.Now(), false)
}
