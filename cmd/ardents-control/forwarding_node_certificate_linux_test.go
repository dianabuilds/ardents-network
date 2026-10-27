package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

func TestForwardingCertificateHasOnlyExactNodeTransportProfile(t *testing.T) {
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	now := time.Now().UTC().Truncate(time.Second)
	facts := epoch.InitialClosedRecord{PublicKey: public, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(time.Hour)}
	raw, err := createForwardingNodeCertificate(facts, key)
	if err != nil {
		t.Fatal(err)
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 || verifyForwardingNodeCertificate(block.Bytes, facts) != nil {
		t.Fatal("actual generated certificate violates closed profile")
	}
	for _, mode := range []string{"subject", "dns", "ip", "ca", "usage", "interval", "key", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			leaf := &x509.Certificate{SerialNumber: big.NewInt(7), NotBefore: facts.ValidFrom, NotAfter: facts.ValidUntil, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, BasicConstraintsValid: true}
			signer := key
			peer := public
			switch mode {
			case "subject":
				leaf.Subject = pkix.Name{CommonName: "must-not-be-a-node-name"}
			case "dns":
				leaf.DNSNames = []string{"example.invalid"}
			case "ip":
				leaf.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
			case "ca":
				leaf.IsCA = true
			case "usage":
				leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			case "interval":
				leaf.NotAfter = leaf.NotAfter.Add(time.Second)
			case "key":
				peer, signer, err = ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				defer clear(signer)
			}
			der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, peer, signer)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "corrupt" {
				der[len(der)-1] ^= 1
			}
			if verifyForwardingNodeCertificate(der, facts) == nil {
				t.Fatal("foreign or extra certificate profile accepted", mode)
			}
		})
	}
}
