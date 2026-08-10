package siteexperiment

import (
	"bytes"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestResolutionUsesTwoBoundQueriesAndRejectsReplay(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_786_387_200, 0)
	fixture, err := newAuthorityFixture("gatec-run", "gatec-network", now, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := newResolutionGateway(fixture, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	key, gateway, err := newOHTTPGateway(resolver)
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewServer(gateway)
	t.Cleanup(gatewayServer.Close)
	relay := testOHTTPRelay(t, gatewayServer.URL, false, nil)
	t.Cleanup(relay.Close)
	transport, err := newOHTTPTransport(key, relay.URL, &http.Client{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	nameData, nameNonce, err := resolveMessage(t.Context(), transport, "name", "site.reference", fixture.runID, fixture.networkID, now)
	if err != nil {
		t.Fatal(err)
	}
	name, err := verifyNameRecord(nameData, fixture.namePublic, fixture.runID, fixture.networkID, nameNonce, now)
	if err != nil {
		t.Fatal(err)
	}
	descriptorData, descriptorNonce, err := resolveMessage(t.Context(), transport, "reachability", name.Target, fixture.runID, fixture.networkID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyDescriptor(descriptorData, fixture.servicePublic, fixture.runID, fixture.networkID, descriptorNonce, name.Target, 1, now); err != nil {
		t.Fatal(err)
	}
	query, _, err := makeResolutionQuery("name", "site.reference", fixture.runID, fixture.networkID, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sendOHTTPMessage(t.Context(), transport, query); err != nil {
		t.Fatal(err)
	}
	if _, err := sendOHTTPMessage(t.Context(), transport, bytes.Clone(query)); err == nil {
		t.Fatal("Gateway accepted a replayed OHTTP lookup")
	}
}
