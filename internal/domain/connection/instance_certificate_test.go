package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestInstanceCertificateRequiresExactSingleEd25519Key(t *testing.T) {
	t.Parallel()
	expected := [32]byte{9}
	certificate := &x509.Certificate{PublicKey: ed25519.PublicKey(expected[:])}
	valid := tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate}}
	if err := verifyInstanceCertificate(valid, expected); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name  string
		state tls.ConnectionState
		key   [32]byte
	}{
		{"absent expected key", valid, [32]byte{}},
		{"foreign expected key", valid, [32]byte{8}},
		{"missing certificate", tls.ConnectionState{Version: tls.VersionTLS13}, expected},
		{"nil certificate", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{nil}}, expected},
		{"multiple certificates", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate, certificate}}, expected},
		{"TLS 1.2", tls.ConnectionState{Version: tls.VersionTLS12, PeerCertificates: valid.PeerCertificates}, expected},
		{"resumed", tls.ConnectionState{Version: tls.VersionTLS13, DidResume: true, PeerCertificates: valid.PeerCertificates}, expected},
		{"wrong algorithm", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: &rsa.PublicKey{}}}}, expected},
		{"short key", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: ed25519.PublicKey{9}}}}, expected},
	} {
		t.Run(input.name, func(t *testing.T) {
			if err := verifyInstanceCertificate(input.state, input.key); err == nil {
				t.Fatal("foreign or unsupported Instance certificate accepted")
			}
		})
	}
}

// This isolates certificate possession and the exact-key TLS verifier. The
// ephemeral fixture is not Publication/Execution authority or a genuine JOIN.
func TestRealTLSHandshakeRejectsForeignInstance(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], public)
	expected[0] ^= 1
	rawClient, rawPublisher := net.Pipe()
	defer rawClient.Close()
	defer rawPublisher.Close()
	if err := rawClient.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := rawPublisher.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	client := tls.Client(rawClient, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true,
		SessionTicketsDisabled: true, CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		VerifyConnection: func(state tls.ConnectionState) error { return verifyInstanceCertificate(state, expected) },
	})
	publisher := tls.Server(rawPublisher, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, SessionTicketsDisabled: true,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		Certificates:     []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: private}},
	})
	joined := make(chan error, 1)
	go func() { joined <- publisher.HandshakeContext(t.Context()) }()
	clientErr := client.HandshakeContext(t.Context())
	// Interrupt the exact fixture peer and join its original handshake before
	// ending the test, whether the verifier rejected or accidentally accepted.
	rawClient.Close()
	publisherErr := <-joined
	if clientErr == nil || client.ConnectionState().HandshakeComplete {
		t.Fatalf("foreign Instance completed Service TLS: peer=%v", publisherErr)
	}
}
