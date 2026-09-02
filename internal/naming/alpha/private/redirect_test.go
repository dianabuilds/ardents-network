package private

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/openpcc/ohttp"
)

func TestPrivateAlphaResolutionRejectsRelayRedirect(t *testing.T) {
	now := time.Unix(2_000_400_000, 0).UTC()
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	network := [32]byte{1}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings:  []alpha.BindingInput{{Link: link, Target: [32]byte{9}}},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := NewGateway(GatewayConfig{Corpus: corpus, NodeID: [32]byte{2}, Family: "gateway-a",
		AssignmentNotAfter: now.Add(time.Hour), IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	defer gatewayServer.Close()
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	redirected.TLS = gatewayServer.TLS.Clone()
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	trusted.TLS = gatewayServer.TLS.Clone()
	trusted.StartTLS()
	defer trusted.Close()

	floor, err := alpha.NewSessionFloor("closed-alpha-1")
	if err != nil {
		t.Fatal(err)
	}
	client, err := Open(ClientConfig{RelayURL: trusted.URL, RelayNodeID: [32]byte{3}, RelayFamily: "relay-b",
		GatewayPublic: gatewayPublic, Gateway: gateway.Profile(), AuthorityPublic: authorityPublic,
		Cohort: "closed-alpha-1", Network: network, Floor: floor,
		Base: gatewayServer.Client().Transport.(*http.Transport).Clone()}, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Resolve(t.Context(), link, now)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("alpha Relay redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("alpha private resolution accepted a Relay redirect")
	}
}

func TestPrivateAlphaRelayRejectsGatewayRedirectForCallerOwnedClientWithoutMutatingIt(t *testing.T) {
	certificateSource := httptest.NewTLSServer(http.NotFoundHandler())
	defer certificateSource.Close()
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	redirected.TLS = certificateSource.TLS.Clone()
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	trusted.TLS = certificateSource.TLS.Clone()
	trusted.StartTLS()
	defer trusted.Close()

	client := trusted.Client()
	if client.CheckRedirect != nil {
		t.Fatal("test caller client unexpectedly rejects redirects")
	}
	relay, err := NewRelay(trusted.URL, client)
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
