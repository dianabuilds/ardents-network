package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/dianabuilds/ardents-network/internal/route"
	"math/big"
	"net"
	"testing"
	"time"
)

func closedTokenListenerCertificate(t *testing.T) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "ardents closed issuer test"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private, Leaf: &template}
	var server [32]byte
	copy(server[:], public)
	return certificate, server
}

func closedTokenListenerEndpoint(t *testing.T, carrier route.CarrierProfile) string {
	t.Helper()
	if carrier == route.ClosedCarrierQUIC {
		listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		if err != nil {
			t.Fatal(err)
		}
		endpoint := listener.LocalAddr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		return endpoint
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}
