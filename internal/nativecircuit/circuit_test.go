package nativecircuit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestTelescopedCircuitAuthenticatesEveryLayerAndCarriesOpaqueStream(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry := newTestNode(t, "entry")
	interior := newTestNode(t, "interior")
	terminal := newTestNode(t, "rendezvous")
	terminalDone := make(chan error, 1)
	go func() {
		terminalDone <- serveOneNode(ctx, terminal.listener, terminal.certificate, func(connection net.Conn) error {
			request, err := readFrame(connection)
			if err != nil {
				return err
			}
			return writeFrame(connection, frame{Type: frameProtectedData, Payload: append([]byte("echo:"), request.Payload...)})
		})
	}()
	interiorDone := make(chan relayObservation, 1)
	go func() {
		interiorDone <- serveOneRelay(ctx, interior.listener, interior.certificate, []string{terminal.address})
	}()
	entryDone := make(chan relayObservation, 1)
	go func() {
		entryDone <- serveOneRelay(ctx, entry.listener, entry.certificate, []string{interior.address})
	}()

	connection, err := dialTelescopedCircuit(ctx, []circuitHop{
		{Address: entry.address, CertificateSHA256: entry.digest},
		{Address: interior.address, CertificateSHA256: interior.digest},
		{Address: terminal.address, CertificateSHA256: terminal.digest},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := writeFrame(connection, frame{Type: frameProtectedData, Payload: []byte("canary")}); err != nil {
		t.Fatal(err)
	}
	response, err := readFrame(connection)
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Payload) != "echo:canary" {
		t.Fatalf("unexpected protected response %q", response.Payload)
	}
	_ = connection.Close()
	if err := <-terminalDone; err != nil {
		t.Fatal(err)
	}
	entryView := <-entryDone
	interiorView := <-interiorDone
	if entryView.NextAddress != interior.address || interiorView.NextAddress != terminal.address {
		t.Fatalf("relay views were not adjacent: entry=%#v interior=%#v", entryView, interiorView)
	}
	if entryView.NextAddress == terminal.address {
		t.Fatal("entry learned the terminal address")
	}
}

type testNode struct {
	listener    net.Listener
	address     string
	certificate tls.Certificate
	digest      [32]byte
}

func newTestNode(t *testing.T, name string) testNode {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{nodeServerName}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: privateKey}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return testNode{listener: listener, address: listener.Addr().String(), certificate: certificate, digest: sha256.Sum256(der)}
}
