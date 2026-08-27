package service_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// referenceC2CertificateMaterial represents the ephemeral identity material
// that the isolated Reference C-2 process fixture gives to one role.
type referenceC2CertificateMaterial struct {
	public      [32]byte
	certificate string
	privateKey  string
}

func referenceC2Certificate(t *testing.T, serial int64, name string) referenceC2CertificateMaterial {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	var material referenceC2CertificateMaterial
	copy(material.public[:], public)
	material.certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	material.privateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	return material
}

func referenceC2Address(t *testing.T) string {
	return referenceC2Addresses(t, 1)[0]
}

// referenceC2Addresses reserves the complete batch before returning it. Asking
// the kernel for one address at a time can hand the just-closed port back to a
// later role, causing two fixture roles to receive the same endpoint.
func referenceC2Addresses(t *testing.T, count int) []string {
	t.Helper()
	listeners := make([]net.Listener, 0, count)
	addresses := make([]string, 0, count)
	for index := 0; index < count; index++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, held := range listeners {
				_ = held.Close()
			}
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
		addresses = append(addresses, listener.Addr().String())
	}
	for _, listener := range listeners {
		_ = listener.Close()
	}
	return addresses
}

func TestReferenceC2AddressesAreDistinctWithinOneFixture(t *testing.T) {
	addresses := referenceC2Addresses(t, 5)
	seen := make(map[string]struct{}, len(addresses))
	for _, address := range addresses {
		if _, duplicate := seen[address]; duplicate {
			t.Fatalf("reference C2 fixture reused endpoint address %q", address)
		}
		seen[address] = struct{}{}
	}
}

func referenceC2Peer(nodeID [32]byte, material referenceC2CertificateMaterial, endpoint string) map[string]string {
	return map[string]string{"NodeID": referenceC2Hex(nodeID), "PublicKey": referenceC2Hex(material.public), "Endpoint": endpoint,
		"Certificate": material.certificate, "PrivateKey": material.privateKey}
}

func referenceC2TransitCredential(t *testing.T, authority ed25519.PrivateKey, network, digest [32]byte, epoch uint64, transitNode [32]byte,
	role byte, attachment [32]byte, deadline time.Time, marker byte) map[string]string {
	t.Helper()
	material := referenceC2Certificate(t, int64(marker), "transit-client")
	certificate, err := tls.X509KeyPair([]byte(material.certificate), []byte(material.privateKey))
	if err != nil || len(certificate.Certificate) != 1 {
		t.Fatal("create transit client certificate")
	}
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	digestKey, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), GrantID: referenceC2ID(marker),
		NetworkID: network, Digest: digest, AttachmentID: attachment, TransitNodeID: transitNode, ClientKeyDigest: digestKey,
		Epoch: epoch, TransitRole: role, NotAfter: deadline}, authority)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"Grant": base64.RawStdEncoding.EncodeToString(grant), "Certificate": material.certificate, "PrivateKey": material.privateKey}
}

func referenceC2EntryInvite(t *testing.T, material referenceC2CertificateMaterial, network, digest [32]byte, epoch uint64, candidate entry.Candidate,
	deadline, now time.Time) []byte {
	t.Helper()
	certificate, err := tls.X509KeyPair([]byte(material.certificate), []byte(material.privateKey))
	if err != nil {
		t.Fatal(err)
	}
	signer, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("fixture Initiator private key is invalid")
	}
	raw, err := entry.Issue(entry.IssueInput{NetworkID: network, Digest: digest, Epoch: epoch,
		Candidate: candidate,
		NotBefore: now.Add(-time.Second), NotAfter: deadline, Slot: 0, Generation: 1}, signer)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
