package reachability_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/openpcc/ohttp"
)

func TestPrivateReachabilityDirectAdapterRejectsRelayRedirect(t *testing.T) {
	now := time.Unix(2_000_300_000, 0).UTC()
	certificate, _ := gatewayRedirectCertificate(t)
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		redirectedRequests.Add(1)
		writer.Header().Set("Content-Type", ohttp.ResponseMediaType)
		_, _ = writer.Write([]byte("opaque-ohttp-response"))
	}))
	redirected.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	trusted.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	trusted.StartTLS()
	defer trusted.Close()

	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], gatewayPublic)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: [32]byte{1}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: [32]byte{1}, NodeID: [32]byte{2}, IdentityKey: gatewayPrivate,
		AssignmentNotAfter: now.Add(time.Minute), Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(reachability.Descriptor, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	base, ok := trusted.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatal("TLS trusted client does not expose HTTP transport")
	}
	client, err := reachability.OpenClient(reachability.ClientConfig{NetworkID: [32]byte{1}, GatewayPublic: expected,
		Profile: gateway.Profile(), RelayURL: trusted.URL + "/ohttp", BaseTransport: base, At: now, Deadline: now.Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.Resolve(context.Background(), [32]byte{3})
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("direct adapter followed a Relay redirect: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("direct adapter accepted a Relay redirect")
	}
}
