package reachability_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/openpcc/ohttp"
)

func TestForwardOHTTPRejectsGatewayRedirectForCallerOwnedClientWithoutMutatingIt(t *testing.T) {
	certificate, _ := gatewayRedirectCertificate(t)
	envelope := []byte("opaque-ohttp-envelope")
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
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	trusted.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	trusted.StartTLS()
	defer trusted.Close()

	client := trusted.Client()
	if client.CheckRedirect != nil {
		t.Fatal("test caller client unexpectedly rejects redirects")
	}
	_, err := reachability.ForwardOHTTP(context.Background(), trusted.URL, client, envelope)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("caller client Gateway redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("ForwardOHTTP accepted a caller-client Gateway redirect")
	}
	if client.CheckRedirect != nil {
		t.Fatal("ForwardOHTTP mutated the caller client redirect policy")
	}
}
