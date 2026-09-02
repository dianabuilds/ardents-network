package resolution_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	nameresolution "github.com/dianabuilds/ardents-network/internal/naming/resolution"
	"github.com/openpcc/ohttp"
)

func TestPrivateResolutionRejectsSelectedRelayRedirect(t *testing.T) {
	fixture := newResolutionFixture(t)
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	redirected.TLS = fixture.relayServer.TLS.Clone()
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	trusted.TLS = fixture.relayServer.TLS.Clone()
	trusted.StartTLS()
	defer trusted.Close()

	view := fixture.view
	view.Candidates[0].Endpoint = endpointOf(t, trusted.URL)
	selection := fixture.admitted(t, fixture.selection, "alice", [32]byte{7}, 7)
	resolver, err := nameresolution.OpenEvidence(view, selection, fixture.gatewayProfile(), [32]byte{7}, relayTransport(trusted))
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), "alice", fixture.now)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("selected Relay redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil || result.Class != "private resolution unavailable" {
		t.Fatalf("Resolve redirect result = %+v, %v", result, err)
	}
}

func TestPrivateResolutionRelayRejectsGatewayRedirectForCallerOwnedClientWithoutMutatingIt(t *testing.T) {
	fixture := newResolutionFixture(t)
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	redirected.TLS = fixture.relayServer.TLS.Clone()
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	trusted.TLS = fixture.relayServer.TLS.Clone()
	trusted.StartTLS()
	defer trusted.Close()

	client := trusted.Client()
	if client.CheckRedirect != nil {
		t.Fatal("test caller client unexpectedly rejects redirects")
	}
	relay, err := nameresolution.NewRelay(trusted.URL, client)
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	defer relayServer.Close()
	request, err := http.NewRequest(http.MethodPost, relayServer.URL+"/ohttp", bytes.NewReader([]byte{1}))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", ohttp.RequestMediaType)
	response, err := relayServer.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("Gateway redirect was followed: redirect target received %d request(s)", got)
	}
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("Relay redirect response status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	if client.CheckRedirect != nil {
		t.Fatal("NewRelay mutated the caller client redirect policy")
	}
}
