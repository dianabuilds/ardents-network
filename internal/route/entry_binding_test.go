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
	"errors"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

func TestEntryBindingV2CanonicalVector(t *testing.T) {
	input := entryBindingFixture()
	raw, err := EncodeEntryBinding(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d696e7465726163746976652d726f7574652d76320000d60002011c617264656e74732d696e7465726163746976652d726f7574652d76321500000000000000000000000000000000000000000000000000000000000000000000000000001916000000000000000000000000000000000000000000000000000000000000001700000000000000000000000000000000000000000000000000000000000000180000000000000000000000000000000000000000000000000000000000000000000000684ee1801a00000000000000000000000000000000000000000000000000000000000000000401020304"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical EntryBinding vector = %x, want %s", raw, want)
	}
	decoded, err := DecodeEntryBinding(raw)
	if err != nil || !equalEntryBinding(decoded, input) {
		t.Fatalf("decoded EntryBinding = %+v, %v", decoded, err)
	}
}

func TestEntryBindingV2RejectsWrongKindAndMalformedInvite(t *testing.T) {
	raw, err := EncodeEntryBinding(entryBindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	wrongKind := append([]byte(nil), raw...)
	wrongKind[len(routeWireMagic)+2+2] = legBindingKind
	truncated := raw[:len(raw)-1]
	for index, value := range [][]byte{nil, wrongKind, truncated, append(raw, 0)} {
		if _, err := DecodeEntryBinding(value); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
}

func TestEntryBindingV2RejectsRouteV1Identity(t *testing.T) {
	raw, err := EncodeEntryBinding(entryBindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	legacy := append([]byte(nil), raw...)
	copy(legacy, "ardents-interactive-route-v1\x00")
	if _, err := DecodeEntryBinding(legacy); err == nil {
		t.Fatal("Route v1 EntryBinding identity was accepted by Route v2")
	}
}

func TestEntryBindingV2RejectsRouteV1Body(t *testing.T) {
	raw, err := EncodeEntryBinding(entryBindingFixture())
	if err != nil {
		t.Fatal(err)
	}
	legacy := append([]byte(nil), raw...)
	legacy[len(routeWireMagic)+2] = 0
	legacy[len(routeWireMagic)+3] = 1
	if _, err := DecodeEntryBinding(legacy); err == nil {
		t.Fatal("Route v1 EntryBinding body was accepted by Route v2")
	}
}

func TestEntryBindingBindsFreshMutualTLSClientKey(t *testing.T) {
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

func TestAdmitEntryBindingRejectsSubstitutionAndConsumesOneTuple(t *testing.T) {
	certificate := entryBindingCertificate(t, 41)
	digest, err := ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	binding := entryBindingFixture()
	binding.ClientKeyDigest = digest
	admission := EntryAdmission{InviteID: identifier(42), NetworkID: binding.NetworkID, Digest: binding.Digest,
		Epoch: binding.Epoch, InitiatorNodeID: binding.InitiatorNodeID, NotAfter: binding.NotAfter.Add(time.Minute)}
	var lock sync.Mutex
	consumed := map[[96]byte]struct{}{}
	admit := func(raw []byte, attachment, clientKey [32]byte, notAfter time.Time) (EntryAdmission, error) {
		if !bytes.Equal(raw, binding.Invite) {
			return EntryAdmission{}, errors.New("wrong opaque Invite")
		}
		if notAfter != binding.NotAfter {
			return EntryAdmission{}, errors.New("wrong binding expiry")
		}
		var key [96]byte
		copy(key[:32], admission.InviteID[:])
		copy(key[32:64], attachment[:])
		copy(key[64:], clientKey[:])
		lock.Lock()
		defer lock.Unlock()
		if _, exists := consumed[key]; exists {
			return EntryAdmission{}, errors.New("replayed Entry binding")
		}
		consumed[key] = struct{}{}
		return admission, nil
	}
	if err := AdmitEntryBinding(binding, certificate.Leaf, binding.NotAfter.Add(-time.Second), admit); err != nil {
		t.Fatal(err)
	}
	if err := AdmitEntryBinding(binding, certificate.Leaf, binding.NotAfter.Add(-time.Second), admit); err == nil {
		t.Fatal("replayed Entry binding was consumed twice")
	}
	other := entryBindingCertificate(t, 43)
	if err := AdmitEntryBinding(binding, other.Leaf, binding.NotAfter.Add(-time.Second), admit); err == nil {
		t.Fatal("different TLS client key was accepted")
	}
}

func entryBindingFixture() EntryBinding {
	return EntryBinding{NetworkID: identifier(21), Digest: identifier(22), AttachmentID: identifier(23), InitiatorNodeID: identifier(24),
		Epoch: 25, NotAfter: time.Unix(1_750_000_000, 0).UTC(), ClientKeyDigest: identifier(26), Invite: []byte{1, 2, 3, 4}}
}

func equalEntryBinding(left, right EntryBinding) bool {
	return left.NetworkID == right.NetworkID && left.Digest == right.Digest && left.AttachmentID == right.AttachmentID &&
		left.InitiatorNodeID == right.InitiatorNodeID && left.Epoch == right.Epoch && left.NotAfter.Equal(right.NotAfter) &&
		left.ClientKeyDigest == right.ClientKeyDigest && bytes.Equal(left.Invite, right.Invite)
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
