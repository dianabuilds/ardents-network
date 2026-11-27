package connection

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

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
	client := newInstanceTLSClient(rawClient, expected)
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
	if clientErr == nil || !strings.Contains(clientErr.Error(), "key differs from verified Publication") || client.ConnectionState().HandshakeComplete {
		t.Fatalf("foreign Instance was not refused by key binding: client=%v peer=%v", clientErr, publisherErr)
	}
}
