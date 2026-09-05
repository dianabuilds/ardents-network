package source

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSourceTLSVerification(t *testing.T) {
	now := time.Date(2032, time.February, 3, 4, 5, 6, 0, time.UTC)
	for _, test := range []struct {
		name   string
		change func(t *testing.T, fixture *sourceTLSFixture, now time.Time)
		verify func(t *testing.T, clientErr, serverErr error)
	}{
		{name: "accepts valid CA hostname and pins", verify: sourceTLSAccepted},
		{name: "rejects expired server leaf", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			fixture.server = fixture.authority.issueServer(t, now.Add(-2*time.Hour), now.Add(-time.Hour), "source.test")
			fixture.serverPin = sourceTLSLeafKeyDigest(t, fixture.server)
		}, verify: sourceTLSClientExpired},
		{name: "rejects expired client leaf", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			fixture.client = fixture.authority.issueClient(t, now.Add(-2*time.Hour), now.Add(-time.Hour))
			fixture.clientPin = sourceTLSLeafKeyDigest(t, fixture.client)
		}, verify: sourceTLSServerExpired},
		{name: "rejects server leaf before validity", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			fixture.server = fixture.authority.issueServer(t, now.Add(time.Hour), now.Add(2*time.Hour), "source.test")
			fixture.serverPin = sourceTLSLeafKeyDigest(t, fixture.server)
		}, verify: sourceTLSClientExpired},
		{name: "rejects server leaf from unknown CA", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			other := newSourceTLSAuthority(t, now, 101)
			fixture.server = other.issueServer(t, now.Add(-time.Hour), now.Add(time.Hour), "source.test")
			fixture.serverPin = sourceTLSLeafKeyDigest(t, fixture.server)
		}, verify: sourceTLSClientUnknownAuthority},
		{name: "rejects trusted server with unpinned key", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			fixture.server = fixture.authority.issueServer(t, now.Add(-time.Hour), now.Add(time.Hour), "source.test")
			fixture.serverPin = sourceTLSLeafKeyDigest(t, fixture.authority.issueServer(t, now.Add(-time.Hour), now.Add(time.Hour), "source.test"))
		}, verify: sourceTLSClientServerPin},
		{name: "rejects trusted client with unauthorized key", change: func(t *testing.T, fixture *sourceTLSFixture, now time.Time) {
			fixture.client = fixture.authority.issueClient(t, now.Add(-time.Hour), now.Add(time.Hour))
			fixture.clientPin = sourceTLSLeafKeyDigest(t, fixture.authority.issueClient(t, now.Add(-time.Hour), now.Add(time.Hour)))
		}, verify: sourceTLSServerClientPin},
		{name: "rejects mismatched server hostname", change: func(t *testing.T, fixture *sourceTLSFixture, _ time.Time) {
			fixture.serverName = "other-source.test"
		}, verify: sourceTLSClientHostname},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newSourceTLSFixture(t, now)
			if test.change != nil {
				test.change(t, fixture, now)
			}
			clientErr, serverErr := fixture.handshake(t, now)
			test.verify(t, clientErr, serverErr)
		})
	}
}

type sourceTLSFixture struct {
	authority  sourceTLSAuthority
	server     tls.Certificate
	client     tls.Certificate
	serverName string
	serverPin  [32]byte
	clientPin  [32]byte
}

type sourceTLSAuthority struct {
	certificate *x509.Certificate
	private     ed25519.PrivateKey
	pool        *x509.CertPool
	nextSeed    byte
}

func newSourceTLSFixture(t *testing.T, now time.Time) *sourceTLSFixture {
	t.Helper()
	authority := newSourceTLSAuthority(t, now, 1)
	fixture := &sourceTLSFixture{authority: authority, serverName: "source.test"}
	fixture.server = authority.issueServer(t, now.Add(-time.Hour), now.Add(time.Hour), fixture.serverName)
	fixture.client = authority.issueClient(t, now.Add(-time.Hour), now.Add(time.Hour))
	fixture.serverPin = sourceTLSLeafKeyDigest(t, fixture.server)
	fixture.clientPin = sourceTLSLeafKeyDigest(t, fixture.client)
	return fixture
}

func newSourceTLSAuthority(t *testing.T, now time.Time, seed byte) sourceTLSAuthority {
	t.Helper()
	public, private := sourceTLSTestKey(seed)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-24 * time.Hour), NotAfter: now.Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(sourceTLSTestReader{}, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificate)
	return sourceTLSAuthority{certificate: certificate, private: private, pool: pool, nextSeed: seed + 1}
}

func (authority *sourceTLSAuthority) issueServer(t *testing.T, notBefore, notAfter time.Time, hostname string) tls.Certificate {
	t.Helper()
	return authority.issueLeaf(t, notBefore, notAfter, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, []string{hostname}, 2)
}

func (authority *sourceTLSAuthority) issueClient(t *testing.T, notBefore, notAfter time.Time) tls.Certificate {
	t.Helper()
	return authority.issueLeaf(t, notBefore, notAfter, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil, 3)
}

func (authority *sourceTLSAuthority) issueLeaf(t *testing.T, notBefore, notAfter time.Time, usages []x509.ExtKeyUsage, names []string, serial int64) tls.Certificate {
	t.Helper()
	public, private := sourceTLSTestKey(authority.nextSeed)
	authority.nextSeed++
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: notBefore, NotAfter: notAfter, DNSNames: names,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usages}
	raw, err := x509.CreateCertificate(sourceTLSTestReader{}, template, authority.certificate, public, authority.private)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}
}

func sourceTLSTestKey(value byte) (ed25519.PublicKey, ed25519.PrivateKey) {
	var seed [ed25519.SeedSize]byte
	for index := range seed {
		seed[index] = value + byte(index)
	}
	private := ed25519.NewKeyFromSeed(seed[:])
	return private.Public().(ed25519.PublicKey), private
}

type sourceTLSTestReader struct{}

func (sourceTLSTestReader) Read(raw []byte) (int, error) {
	clear(raw)
	return len(raw), nil
}

func sourceTLSLeafKeyDigest(t *testing.T, certificate tls.Certificate) [32]byte {
	t.Helper()
	digest, err := certificateDigest(certificate)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func (fixture *sourceTLSFixture) handshake(t *testing.T, now time.Time) (clientErr, serverErr error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverConfig := serverTLSConfig(server{certificate: fixture.server, clientRoots: fixture.authority.pool,
		clientDigests: map[[32]byte]bool{fixture.clientPin: true}})
	clientConfig := clientTLSConfig(client{address: listener.Addr().String(), serverName: fixture.serverName,
		roots: fixture.authority.pool, leafKeyDigest: fixture.serverPin, certificate: fixture.client})
	serverConfig.Time, clientConfig.Time = func() time.Time { return now }, func() time.Time { return now }
	serverContext, cancelServer := context.WithTimeout(t.Context(), time.Second)
	defer cancelServer()

	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		tlsConnection := tls.Server(connection, serverConfig)
		serverDone <- tlsConnection.HandshakeContext(serverContext)
	}()

	connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	tlsConnection := tls.Client(connection, clientConfig)
	clientContext, cancelClient := context.WithTimeout(t.Context(), time.Second)
	clientErr = tlsConnection.HandshakeContext(clientContext)
	cancelClient()
	if closeErr := tlsConnection.Close(); closeErr != nil && clientErr == nil {
		clientErr = closeErr
	}
	select {
	case serverErr = <-serverDone:
	case <-serverContext.Done():
		t.Fatal("TLS server handshake did not finish before its deadline")
	}
	return clientErr, serverErr
}

func sourceTLSAccepted(t *testing.T, clientErr, serverErr error) {
	t.Helper()
	if clientErr != nil || serverErr != nil {
		t.Fatalf("valid TLS handshake = client %v, server %v", clientErr, serverErr)
	}
}

func sourceTLSClientExpired(t *testing.T, clientErr, _ error) {
	t.Helper()
	sourceTLSCertificateExpired(t, clientErr)
}

func sourceTLSServerExpired(t *testing.T, _, serverErr error) {
	t.Helper()
	sourceTLSCertificateExpired(t, serverErr)
}

func sourceTLSCertificateExpired(t *testing.T, err error) {
	t.Helper()
	var invalid x509.CertificateInvalidError
	if !errors.As(err, &invalid) || invalid.Reason != x509.Expired {
		t.Fatalf("certificate validity error = %v, want x509.CertificateInvalidError(%v)", err, x509.Expired)
	}
}

func sourceTLSClientUnknownAuthority(t *testing.T, clientErr, _ error) {
	t.Helper()
	var unknown x509.UnknownAuthorityError
	if !errors.As(clientErr, &unknown) {
		t.Fatalf("unknown server CA = %v, want x509.UnknownAuthorityError", clientErr)
	}
}

func sourceTLSClientServerPin(t *testing.T, clientErr, _ error) {
	t.Helper()
	sourceTLSErrorContains(t, clientErr, "distribution source leaf key pin does not match")
}

func sourceTLSServerClientPin(t *testing.T, _, serverErr error) {
	t.Helper()
	sourceTLSErrorContains(t, serverErr, "distribution client leaf key pin is not authorized")
}

func sourceTLSClientHostname(t *testing.T, clientErr, _ error) {
	t.Helper()
	var hostname x509.HostnameError
	if !errors.As(clientErr, &hostname) {
		t.Fatalf("server hostname mismatch = %v, want x509.HostnameError", clientErr)
	}
}

func sourceTLSErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("TLS verification error = %v, want %q", err, expected)
	}
}
