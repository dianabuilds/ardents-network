//go:build linux

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func certificateDeclaration(t *testing.T, record any, reference time.Time) []byte {
	t.Helper()
	return preparationJSON(t, map[string]any{"schema": "ardents-node-certificate-plan-v1", "reference_time": reference.Format(time.RFC3339Nano), "record": record})
}

func TestNetworkSigningNodeCertificateSemantics(t *testing.T) {
	p, key, message, now := signingRecordFixture(t)
	now = time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	_, message, err := prepareNetworkPublic("prepare-initial-node-record", preparationJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	signed := append(bytes.Clone(message), ed25519.Sign(key, message)...)
	digest := sha256.Sum256(signed)
	raw := certificateDeclaration(t, p, now)
	s, err := admitNetworkSigning("prepare-node-certificate", raw, signed, preparationHex(digest[:]))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := s.sign(key, now)
	if err != nil || len(artifacts) != 1 || artifacts[0].name != "node-certificate.pem" {
		t.Fatal("leaf output", err)
	}
	if output, err := s.nodeCertificateWithRandom(key, bytes.NewReader(nil)); output != nil || !errors.Is(err, io.EOF) {
		t.Fatal("entropy failure lost", err)
	}
	block, rest := pem.Decode(artifacts[0].raw)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 || len(block.Headers) != 0 {
		t.Fatal("single PEM")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	from, _ := admissionTime(p["valid_from"].(string))
	until, _ := admissionTime(p["valid_until"].(string))
	// Independent oracle consumes actual DER, not the generator template or its
	// verifier. No deterministic serial/DER expectation is imposed.
	if !bytes.Equal(c.PublicKey.(ed25519.PublicKey), key.Public().(ed25519.PublicKey)) ||
		!c.NotBefore.Equal(from) || !c.NotAfter.Equal(until) || c.IsCA || !c.BasicConstraintsValid ||
		c.KeyUsage != x509.KeyUsageDigitalSignature || len(c.ExtKeyUsage) != 2 ||
		c.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || c.ExtKeyUsage[1] != x509.ExtKeyUsageClientAuth ||
		!bytes.Equal(c.RawSubject, []byte{48, 0}) || !bytes.Equal(c.RawIssuer, c.RawSubject) || len(c.Extensions) != 3 ||
		!ed25519.Verify(key.Public().(ed25519.PublicKey), c.RawTBSCertificate, c.Signature) {
		t.Fatal("independent public leaf semantics")
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(der)
	private := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	defer clear(private)
	if _, err := tls.X509KeyPair(artifacts[0].raw, private); err != nil {
		t.Fatal("runtime key-pair compatibility", err)
	}
	altered := bytes.Clone(block.Bytes)
	altered[len(altered)-1] ^= 1
	if s.verifyNodeCertificate(altered) == nil {
		t.Fatal("altered certificate signature accepted")
	}
	peer := tls.Certificate{Certificate: [][]byte{block.Bytes}, PrivateKey: key}
	var pin [32]byte
	copy(pin[:], key[32:])
	testNodeCertificatePeer(t, peer, pin, "ardents-route-v3", true)
	wrongPin := pin
	wrongPin[0] ^= 1
	testNodeCertificatePeer(t, peer, wrongPin, "ardents-route-v3", false)
	testNodeCertificatePeer(t, peer, pin, "foreign-alpn", false)
	// A separately forged negative peer leaf cannot turn the accepted key into
	// live validity. The healthy certificate above always came from the producer.
	expired := *c
	expired.NotBefore, expired.NotAfter = now.Add(-2*time.Hour), now.Add(-time.Hour)
	expiredDER, err := x509.CreateCertificate(rand.Reader, &expired, &expired, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	if s.verifyNodeCertificate(expiredDER) == nil {
		t.Fatal("changed interval accepted")
	}
	testNodeCertificatePeer(t, tls.Certificate{Certificate: [][]byte{expiredDER}, PrivateKey: key}, pin, "ardents-route-v3", false)
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, 32))
	defer clear(wrong)
	if _, err := s.sign(wrong, now); err == nil {
		t.Fatal("wrong original key accepted")
	}
	for _, fault := range []string{"signature", "bytes", "trailing-record", "pin", "purpose", "csr", "ca", "carrier", "declaration-key", "fraction", "duplicate", "trailing-plan"} {
		t.Run(fault, func(t *testing.T) {
			r, record, pin := bytes.Clone(raw), bytes.Clone(signed), preparationHex(digest[:])
			switch fault {
			case "signature":
				record[len(record)-1] ^= 1
				d := sha256.Sum256(record)
				pin = preparationHex(d[:])
			case "bytes":
				record[6] ^= 1
				d := sha256.Sum256(record)
				pin = preparationHex(d[:])
			case "trailing-record":
				record = append(record, 0)
				d := sha256.Sum256(record)
				pin = preparationHex(d[:])
			case "pin":
				pin = preparationIdentifier(99)
			case "purpose":
				r = signingDeclaration(t, "node-record", p, now)
			case "csr":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"csr":"request","record":`), 1)
			case "ca":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"ca":true,"record":`), 1)
			case "carrier":
				r = bytes.ReplaceAll(r, []byte("ardents-carrier-tcp-tls-v2"), []byte("ardents-carrier-quic-v2"))
			case "declaration-key":
				r = bytes.Replace(r, []byte(preparationHex(key[32:])), []byte(preparationHex(wrong[32:])), 1)
			case "fraction":
				r = certificateDeclaration(t, p, now.Add(time.Nanosecond))
			case "duplicate":
				r = bytes.Replace(r, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1)
			case "trailing-plan":
				r = append(r, []byte(" null")...)
			}
			if _, err := admitNetworkSigning("prepare-node-certificate", r, record, pin); err == nil {
				t.Fatal("invalid proof accepted")
			}
			if _, err := s.sign(key, now); err != nil {
				t.Fatal("healthy control", err)
			}
		})
	}
	for _, actual := range []time.Time{now.Add(-time.Minute - time.Nanosecond), now.Add(5*time.Minute + time.Nanosecond), until, from.Add(-time.Nanosecond)} {
		if _, err := s.sign(key, actual); err == nil {
			t.Fatal("invalid original time admitted")
		}
	}
}

func testNodeCertificatePeer(t *testing.T, certificate tls.Certificate, pin [32]byte, alpn string, accepted bool) {
	t.Helper()
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	deadline := time.Now().Add(5 * time.Second)
	if err := remote.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	server := tls.Server(remote, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, NextProtos: []string{alpn}, SessionTicketsDisabled: true})
	joined := make(chan error, 1)
	go func() { joined <- server.HandshakeContext(t.Context()) }()
	client, err := routeFixtureRoleTLS(t, local, pin, deadline)
	if (err == nil) != accepted {
		_ = local.Close()
		_ = remote.Close()
		<-joined
		t.Fatal("actual exact-key/ALPN peer verdict", err)
	}
	if client != nil && (client.ConnectionState().Version != tls.VersionTLS13 || client.ConnectionState().NegotiatedProtocol != "ardents-route-v3") {
		t.Fatal("wrong peer TLS profile")
	}
	_ = local.Close()
	_ = remote.Close()
	serverErr := <-joined
	if accepted && serverErr != nil {
		t.Fatal("actual peer handshake", serverErr)
	}
}
