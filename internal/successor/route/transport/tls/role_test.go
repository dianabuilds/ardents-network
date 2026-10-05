package tls_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"io"
	"math/big"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/successor/route/transport/tls"
)

func roleCertificate(t *testing.T, start, end time.Time) (stdtls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(private) })
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: start, NotAfter: end,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var key [32]byte
	copy(key[:], public)
	return stdtls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, key
}

type roleAcceptance struct {
	connection *stdtls.Conn
	err        error
}

// Real TLS runs over a pipe. These tests exercise transport authentication and
// bytes only; the certificate fixture supplies no Network or Admission right.
func roleServer(t *testing.T, certificate stdtls.Certificate) (net.Conn, <-chan roleAcceptance, time.Time) {
	t.Helper()
	local, remote := net.Pipe()
	end := time.Now().Add(3 * time.Second)
	result := make(chan roleAcceptance, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		connection, err := roletls.AcceptRole(t.Context(), remote, certificate, end)
		result <- roleAcceptance{connection, err}
	}()
	t.Cleanup(func() { _ = local.Close(); _ = remote.Close(); <-joined })
	return local, result, end
}

func TestRoleTLSAuthenticatesExactPeerAndCarriesBytes(t *testing.T) {
	now := time.Now()
	certificate, key := roleCertificate(t, now.Add(-time.Minute), now.Add(time.Minute))
	raw, accepted, end := roleServer(t, certificate)
	client, err := roletls.OpenRole(t.Context(), raw, key, end)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	if server.err != nil {
		t.Fatal(server.err)
	}
	clientState, serverState := client.ConnectionState(), server.connection.ConnectionState()
	if clientState.Version != stdtls.VersionTLS13 || serverState.Version != stdtls.VersionTLS13 ||
		clientState.NegotiatedProtocol != "ardents-route-v3" || serverState.NegotiatedProtocol != "ardents-route-v3" ||
		len(clientState.PeerCertificates) != 1 || len(serverState.PeerCertificates) != 0 {
		t.Fatal("role identity, version or exact profile differs")
	}
	clientExport, err := clientState.ExportKeyingMaterial("EXPORTER-route-adapter-test", nil, 32)
	if err != nil {
		t.Fatal(err)
	}
	serverExport, err := serverState.ExportKeyingMaterial("EXPORTER-route-adapter-test", nil, 32)
	if err != nil || !bytes.Equal(clientExport, serverExport) {
		t.Fatal("authenticated exporters differ", err)
	}
	if err := client.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	if err := server.connection.SetDeadline(end); err != nil {
		t.Fatal(err)
	}
	payload := []byte("opaque transport bytes")
	read := make(chan error, 1)
	go func() {
		got := make([]byte, len(payload))
		_, err := io.ReadFull(server.connection, got)
		if err == nil && !bytes.Equal(got, payload) {
			err = io.ErrUnexpectedEOF
		}
		read <- err
	}()
	_, writeErr := client.Write(payload)
	readErr := <-read
	if writeErr != nil || readErr != nil {
		t.Fatal("actual ordered bytes failed", writeErr, readErr)
	}
}

func TestRoleTLSRefusesWrongKeyAndCertificateTimes(t *testing.T) {
	for _, refusal := range []string{"wrong-key", "expired", "future"} {
		t.Run(refusal, func(t *testing.T) {
			now := time.Now()
			start, end := now.Add(-time.Minute), now.Add(time.Minute)
			if refusal == "expired" {
				end = now.Add(-time.Minute)
			}
			if refusal == "future" {
				start = now.Add(time.Minute)
			}
			certificate, key := roleCertificate(t, start, end)
			if refusal == "wrong-key" {
				key[0] ^= 1
			}
			raw, accepted, bound := roleServer(t, certificate)
			connection, err := roletls.OpenRole(t.Context(), raw, key, bound)
			if connection != nil || err == nil {
				t.Fatal("invalid peer accepted")
			}
			_ = raw.Close()
			if result := <-accepted; result.err == nil {
				t.Fatal("rejected handshake completed at the peer")
			}
		})
	}
}

type observedRoleConnection struct {
	net.Conn
	writes atomic.Int32
}

func (c *observedRoleConnection) Write(p []byte) (int, error) {
	c.writes.Add(1)
	return c.Conn.Write(p)
}

func TestRoleTLSInvalidBoundsRefuseBeforeOutput(t *testing.T) {
	for _, refusal := range []string{"nil-context", "zero-peer", "expired"} {
		t.Run(refusal, func(t *testing.T) {
			local, remote := net.Pipe()
			defer local.Close()
			defer remote.Close()
			raw := &observedRoleConnection{Conn: local}
			ctx := context.Background()
			key := [32]byte{1}
			end := time.Now().Add(time.Second)
			switch refusal {
			case "nil-context":
				ctx = nil
			case "zero-peer":
				key = [32]byte{}
			case "expired":
				end = time.Now().Add(-time.Second)
			}
			connection, err := roletls.OpenRole(ctx, raw, key, end)
			if connection != nil || err == nil || raw.writes.Load() != 0 {
				t.Fatal("invalid bounds reached output")
			}
		})
	}
	if connection, err := roletls.OpenRole(t.Context(), nil, [32]byte{1}, time.Now().Add(time.Second)); connection != nil || err == nil {
		t.Fatal("absent raw stream accepted")
	}
}

func TestRoleTLSSharedAuthenticationRejectsWrongProfileAndClientIdentity(t *testing.T) {
	for _, state := range []stdtls.ConnectionState{
		{Version: stdtls.VersionTLS12, NegotiatedProtocol: "ardents-route-v3"},
		{Version: stdtls.VersionTLS13, NegotiatedProtocol: "other"},
		{Version: stdtls.VersionTLS13, NegotiatedProtocol: "ardents-route-v3", PeerCertificates: []*x509.Certificate{{}}},
	} {
		if transport.ValidateRoleTLS(state, [32]byte{}, true) == nil {
			t.Fatal("invalid negotiated role state accepted")
		}
	}
}
