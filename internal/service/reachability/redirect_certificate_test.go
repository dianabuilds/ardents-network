package reachability_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func gatewayRedirectCertificate(t *testing.T) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificateTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "private-gateway.test"},
		NotBefore:    time.Unix(0, 0),
		NotAfter:     time.Unix(4_102_444_800, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	raw, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private}, expected
}
