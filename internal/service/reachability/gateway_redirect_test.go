package reachability_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/openpcc/ohttp"
)

func TestForwardOHTTPRejectsGatewayRedirectWithoutSendingEnvelopeToRedirectTarget(t *testing.T) {
	t.Parallel()
	certificate, expected := gatewayRedirectCertificate(t)
	envelope := []byte("opaque-ohttp-envelope")
	var redirectedRequests atomic.Int32

	redirected := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		redirectedRequests.Add(1)
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read redirected request body: %v", err)
			return
		}
		if request.Method != http.MethodPost || string(body) != string(envelope) {
			t.Errorf("redirected request = %s %q", request.Method, body)
		}
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

	client, err := reachability.GatewayHTTPClient(expected)
	if err != nil {
		t.Fatal(err)
	}
	_, err = reachability.ForwardOHTTP(context.Background(), trusted.URL, client, envelope)
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("Gateway redirect was followed: redirect target received %d request(s)", got)
	}
	if err == nil {
		t.Fatal("ForwardOHTTP accepted a Gateway redirect")
	}
}

func gatewayRedirectCertificate(t *testing.T) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificateTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "private-gateway.test"},
		NotBefore:    time.Unix(0, 0),
		NotAfter:     time.Unix(4_102_444_800, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	raw, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, expected
}
