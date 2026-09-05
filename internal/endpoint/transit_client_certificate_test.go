package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestTransitClientCertificateRequiresGrantLocalEnrollment(t *testing.T) {
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := transitClientTestCertificate(t, 201)
	digest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	grant := route.TransitGrant{IssuerID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), GrantID: transitClientTestID(202),
		NetworkID: transitClientTestID(203), Digest: transitClientTestID(204), AttachmentID: transitClientTestID(205), TransitNodeID: transitClientTestID(206),
		ClientKeyDigest: digest, Epoch: 1, TransitRole: route.IntroductionRole, NotAfter: time.Now().UTC().Add(time.Minute).Truncate(time.Second)}
	raw, err := route.IssueTransitGrant(grant, authority)
	if err != nil {
		t.Fatal(err)
	}
	underTest := &endpoint{transitClients: map[[32]byte]tls.Certificate{grant.GrantID: certificate}}
	got, err := underTest.transitClientCertificate(raw, tls.Certificate{})
	if err != nil || got.PrivateKey == nil || got.Leaf == nil {
		t.Fatalf("enrolled grant credential = %+v, %v", got, err)
	}
	if _, err := (&endpoint{}).transitClientCertificate(raw, tls.Certificate{}); err == nil {
		t.Fatal("unenrolled valid Transit Grant was accepted with a fresh key path")
	}
	other := transitClientTestCertificate(t, 207)
	if _, err := underTest.transitClientCertificate(raw, other); err == nil {
		t.Fatal("caller substituted a credential outside Endpoint local enrollment")
	}
}

func TestTransitClientCertificateRejectsMalformedGrantBeforeChoosingCredential(t *testing.T) {
	underTest := &endpoint{}
	supplied := transitClientTestCertificate(t, 208)
	for name, authorization := range map[string][]byte{
		"empty":     nil,
		"random":    {1, 2, 3, 4},
		"truncated": []byte("ardents-transit-grant-v1"),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := underTest.transitClientCertificate(authorization, supplied)
			if err == nil {
				t.Fatalf("malformed Transit Grant selected credential %+v", got)
			}
		})
	}
}

func transitClientTestID(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value
	}
	return result
}

func transitClientTestCertificate(t *testing.T, serial int64) tls.Certificate {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "endpoint-transit-test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}
}
