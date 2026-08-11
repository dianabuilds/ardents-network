package nativecircuit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestJoinedRouteReportsOnlyAuthenticatedPrefixAtFailurePoint(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	certificate, trust := newEndpointFixture(t, "active-instance")
	user, service := net.Pipe()
	nonce := randomHandle(t)
	serviceDone := make(chan error, 1)
	go func() {
		_, err := runEndpointService(ctx, service, certificate, nonce)
		serviceDone <- err
	}()
	stop := errors.New("fixed failure point")
	observation, err := runEndpointUserWithProgress(ctx, user, trust, nonce, make([]byte, 2*maximumApplicationPayload), func() error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("fixed failure point was not propagated: %v", err)
	}
	if observation.ApplicationBytes != maximumApplicationPayload || observation.ApplicationBytesVerified {
		t.Fatalf("failure evidence accepted bytes past the authenticated prefix: %#v", observation)
	}
	if err := <-serviceDone; err == nil {
		t.Fatal("service reported success after the joined Route was interrupted")
	}
}

func TestJoinedRouteAuthenticatesExactInstanceBeforeApplicationStream(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	certificate, trust := newEndpointFixture(t, "active-instance")
	user, service := net.Pipe()
	nonce := randomHandle(t)
	payload := []byte("verified Application bytes through C-5")
	serviceDone := make(chan endpointObservation, 1)
	serviceErr := make(chan error, 1)
	go func() {
		observation, err := runEndpointService(ctx, service, certificate, nonce)
		serviceDone <- observation
		serviceErr <- err
	}()
	userObservation, err := runEndpointUser(ctx, user, trust, nonce, payload)
	if err != nil {
		t.Fatal(err)
	}
	serviceObservation := <-serviceDone
	if err := <-serviceErr; err != nil {
		t.Fatal(err)
	}
	for role, observation := range map[string]endpointObservation{"user": userObservation, "service": serviceObservation} {
		if !observation.ApplicationBytesVerified || observation.TLSVersion != "TLS1.3" || observation.Curve != "X25519" || observation.SessionResumed {
			t.Fatalf("%s endpoint evidence is incomplete: %#v", role, observation)
		}
	}
}

func TestJoinedRouteRejectsSubstitutedInstanceBeforeApplicationStream(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	activeCertificate, trust := newEndpointFixture(t, "active-instance")
	wrongCertificate, _ := newEndpointFixture(t, "wrong-instance")
	_ = activeCertificate
	user, service := net.Pipe()
	nonce := randomHandle(t)
	serviceDone := make(chan error, 1)
	go func() {
		_, err := runEndpointService(ctx, service, wrongCertificate, nonce)
		serviceDone <- err
	}()
	observation, err := runEndpointUser(ctx, user, trust, nonce, []byte("must not arrive"))
	if err == nil {
		t.Fatal("substituted Instance was accepted")
	}
	if observation.ApplicationBytesVerified {
		t.Fatal("Application bytes were accepted before exact Instance authentication")
	}
	<-serviceDone
}

func newEndpointFixture(t *testing.T, name string) (tls.Certificate, endpointTrust) {
	t.Helper()
	rootPublic, rootPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: "Carrier Lab Target"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootPublic, rootPrivate)
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{endpointServerName}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, root, leafPublic, rootPrivate)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return tls.Certificate{Certificate: [][]byte{leafDER, rootDER}, PrivateKey: leafPrivate}, endpointTrust{Roots: roots, LeafSHA256: sha256.Sum256(leafDER)}
}
