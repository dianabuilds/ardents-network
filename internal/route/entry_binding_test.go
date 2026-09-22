package route

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestEntryBindingV2CanonicalSenderVector(t *testing.T) {
	raw, err := EncodeEntryBinding(entryBindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76320000d60002011c617264656e74732d696e7465726163746976652d726f7574652d76321500000000000000000000000000000000000000000000000000000000000000000000000000001916000000000000000000000000000000000000000000000000000000000000001700000000000000000000000000000000000000000000000000000000000000180000000000000000000000000000000000000000000000000000000000000000000000684ee1801a00000000000000000000000000000000000000000000000000000000000000000401020304"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical EntryBinding vector = %x, want %s", raw, want)
	}
	invalid := entryBindingFixture()
	invalid.Invite = nil
	if _, err := EncodeEntryBinding(invalid); err == nil {
		t.Fatal("EntryBinding sender accepted an empty Invite")
	}
}

func TestEntryBindingUsesFreshMutualTLSClientKeyDigest(t *testing.T) {
	client := entryBindingCertificate(t, 31)
	server := entryBindingCertificate(t, 32)
	clientRaw, serverRaw := net.Pipe()
	serverTLS := tls.Server(serverRaw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{server}, ClientAuth: tls.RequireAnyClientCert, SessionTicketsDisabled: true})
	handshake := make(chan error, 1)
	go func() { handshake <- serverTLS.HandshakeContext(context.Background()) }()
	clientTLS := tls.Client(clientRaw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{client}, InsecureSkipVerify: true, SessionTicketsDisabled: true})
	if err := clientTLS.HandshakeContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	defer clientTLS.Close()
	defer serverTLS.Close()
	want, err := ClientTLSKeyDigest(client.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ClientTLSKeyDigest(serverTLS.ConnectionState().PeerCertificates[0])
	if err != nil || got != want {
		t.Fatalf("peer client key digest = %x, %v; want %x", got, err, want)
	}
	second := entryBindingCertificate(t, 33)
	other, err := ClientTLSKeyDigest(second.Leaf)
	if err != nil || other == want {
		t.Fatalf("fresh replacement key digest = %x, %v", other, err)
	}
}

func entryBindingFixture() EntryBinding {
	return EntryBinding{NetworkID: identifier(21), Digest: identifier(22), AttachmentID: identifier(23), InitiatorNodeID: identifier(24),
		Epoch: 25, NotAfter: time.Unix(1_750_000_000, 0).UTC(), ClientKeyDigest: identifier(26), Invite: []byte{1, 2, 3, 4}}
}

func entryBindingCertificate(t *testing.T, serial int64) tls.Certificate {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(serial)}, ed25519.SeedSize))
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "route.test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}

func identifierFromKey(key ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], key)
	return result
}
