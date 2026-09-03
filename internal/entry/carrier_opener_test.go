package entry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestAcquirePassesStateCandidateToMutualTLSOpener(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	serverPrivate := fixture.private[fixture.candidates[0].KeyID]
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	fixture.candidates[0].Endpoint = listener.Addr().String()
	fixture.view.Candidates[0] = fixture.candidates[0]
	owner, err := Open(fixture.config(entryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if result, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil || result.Class != Accepted {
		t.Fatalf("import = %+v, %v", result, err)
	}
	serverCertificate := certificateFor(t, serverPrivate, 71)
	clientPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
	clientCertificate := certificateFor(t, clientPrivate, 72)
	handshake := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			handshake <- acceptErr
			return
		}
		secured := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
			Certificates: []tls.Certificate{serverCertificate}, ClientAuth: tls.RequireAnyClientCert, SessionTicketsDisabled: true})
		handshake <- secured.HandshakeContext(context.Background())
		_ = secured.Close()
	}()
	connection, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{91}, Deadline: fixture.now.Add(5 * time.Second)},
		func(ctx context.Context, candidate Candidate, presentation Presentation, deadline time.Time) (net.Conn, func() error, bool, error) {
			if candidate.Endpoint != fixture.candidates[0].Endpoint || candidate.PublicKey != fixture.candidates[0].PublicKey {
				return nil, nil, true, errors.New("opener did not receive the authenticated State candidate")
			}
			if presentation.InviteID == [32]byte{} || !bytes.Equal(presentation.Invite, fixture.invite(t, fixture.candidates[0], 0, 1, nil)) {
				return nil, nil, true, errors.New("opener did not receive the exact Entry Invite")
			}
			raw, dialErr := (&net.Dialer{}).DialContext(ctx, "tcp", candidate.Endpoint)
			if dialErr != nil {
				return nil, nil, true, dialErr
			}
			secured := tls.Client(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
				Certificates: []tls.Certificate{clientCertificate}, InsecureSkipVerify: true, SessionTicketsDisabled: true,
				VerifyConnection: exactStatePeer(candidate.PublicKey)})
			if setErr := secured.SetDeadline(deadline); setErr != nil {
				_ = raw.Close()
				return nil, nil, true, setErr
			}
			if handshakeErr := secured.HandshakeContext(ctx); handshakeErr != nil {
				_ = raw.Close()
				return nil, nil, true, handshakeErr
			}
			return secured, secured.Close, true, nil
		})
	if err != nil || connection == nil {
		t.Fatalf("Acquire = %v, %v", connection, err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatalf("mutual TLS handshake = %v", err)
	}
}

func certificateFor(t *testing.T, private ed25519.PrivateKey, serial int64) tls.Certificate {
	t.Helper()
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "entry.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: parsed}
}

func exactStatePeer(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) != 1 {
			return errors.New("peer certificate is unavailable")
		}
		public, found := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !found || !bytes.Equal(public, expected[:]) {
			return errors.New("peer certificate does not match State")
		}
		return nil
	}
}
