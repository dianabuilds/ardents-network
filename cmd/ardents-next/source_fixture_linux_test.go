//go:build linux

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// Independently prepared fixture material never comes from product execution.
func sourceNativeTLS(t *testing.T) (tls.Certificate, tls.Certificate, tls.Certificate, []byte, [32]byte, [32]byte) {
	t.Helper()
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	raw, err := x509.CreateCertificate(rand.Reader, ca, ca, public, key)
	if err != nil {
		t.Fatal(err)
	}
	root := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	issue := func(serial int64, usage x509.ExtKeyUsage) (tls.Certificate, [32]byte) {
		public, leafKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), DNSNames: []string{"source-native.test"}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		raw, err := x509.CreateCertificate(rand.Reader, template, ca, public, key)
		if err != nil {
			t.Fatal(err)
		}
		// Independent oracle for the accepted Source transport-key transcript;
		// Source pins are distinct from ordinary SPKI or Epoch authority digests.
		pin := sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), public...))
		return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: leafKey}, pin
	}
	server, serverPin := issue(2, x509.ExtKeyUsageServerAuth)
	client, clientPin := issue(3, x509.ExtKeyUsageClientAuth)
	unauthorized, _ := issue(4, x509.ExtKeyUsageClientAuth)
	return server, client, unauthorized, root, serverPin, clientPin
}
