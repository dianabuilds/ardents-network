package credential

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/openpcc/ohttp"
)

func TestForwardOHTTPRejectsIssuerRedirectWithoutSendingEnvelopeToRedirectTarget(t *testing.T) {
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, initiatorPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuerCertificate := credentialCertificate(t, issuerPrivate, 1)
	initiatorCertificate := credentialCertificate(t, initiatorPrivate, 2)
	envelope := []byte("opaque-ohttp-envelope")
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		redirectedRequests.Add(1)
		writer.Header().Set("Content-Type", ohttp.ResponseMediaType)
		_, _ = writer.Write([]byte("opaque-ohttp-response"))
	}))
	redirected.TLS = &tls.Config{Certificates: []tls.Certificate{issuerCertificate}}
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	trusted.TLS = &tls.Config{Certificates: []tls.Certificate{issuerCertificate}}
	trusted.StartTLS()
	defer trusted.Close()

	client, err := HTTPClient(publicIdentifier(issuerPublic), initiatorCertificate)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	_, err = ForwardOHTTP(context.Background(), trusted.URL, client, envelope)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("issuer redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("ForwardOHTTP accepted an issuer redirect")
	}
}

func TestForwardOHTTPRejectsIssuerRedirectForCallerOwnedClientWithoutMutatingIt(t *testing.T) {
	_, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	issuerCertificate := credentialCertificate(t, issuerPrivate, 3)
	envelope := []byte("opaque-ohttp-envelope")
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		redirectedRequests.Add(1)
		writer.Header().Set("Content-Type", ohttp.ResponseMediaType)
		_, _ = writer.Write([]byte("opaque-ohttp-response"))
	}))
	redirected.TLS = &tls.Config{Certificates: []tls.Certificate{issuerCertificate}}
	redirected.StartTLS()
	defer redirected.Close()

	trusted := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusTemporaryRedirect)
	}))
	trusted.TLS = &tls.Config{Certificates: []tls.Certificate{issuerCertificate}}
	trusted.StartTLS()
	defer trusted.Close()

	transport := trusted.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.InsecureSkipVerify = true
	client := &http.Client{Transport: transport}
	if client.CheckRedirect != nil {
		t.Fatal("test caller client unexpectedly rejects redirects")
	}
	_, err = ForwardOHTTP(context.Background(), trusted.URL, client, envelope)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("caller client issuer redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("ForwardOHTTP accepted a caller-client issuer redirect")
	}
	if client.CheckRedirect != nil {
		t.Fatal("ForwardOHTTP mutated the caller client redirect policy")
	}
}
