//go:build linux || darwin

package networksigning

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/node/provisioning/networkdraft"
	provisionfixture "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNetworkSigningNodeCertificateSemantics(t *testing.T) {
	p, key, _, _ := provisionfixture.RecordSigningPlan(t)
	now := time.Now().UTC().Truncate(time.Second)
	p["valid_from"], p["valid_until"] = now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)
	_, message, err := networkdraft.Prepare("prepare-initial-node-record", provisionfixture.PlanJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	signed := append(bytes.Clone(message), ed25519.Sign(key, message)...)
	digest := sha256.Sum256(signed)
	raw := provisionfixture.CertificateDeclaration(t, p, now)
	s, err := admitNetworkSigning("prepare-node-certificate", raw, signed, hex.EncodeToString(digest[:]))
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
	from, _ := networkdraft.ParseTime(p["valid_from"].(string))
	until, _ := networkdraft.ParseTime(p["valid_until"].(string))
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
	if verifyIssuedCertificate(s, altered) == nil {
		t.Fatal("altered certificate signature accepted")
	}
	// A separately forged negative peer leaf cannot turn the accepted key into
	// live validity. The healthy certificate above always came from the producer.
	expired := *c
	expired.NotBefore, expired.NotAfter = now.Add(-2*time.Hour), now.Add(-time.Hour)
	expiredDER, err := x509.CreateCertificate(rand.Reader, &expired, &expired, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	if verifyIssuedCertificate(s, expiredDER) == nil {
		t.Fatal("changed interval accepted")
	}
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, 32))
	defer clear(wrong)
	if _, err := s.sign(wrong, now); err == nil {
		t.Fatal("wrong original key accepted")
	}
	for _, fault := range []string{"signature", "bytes", "trailing-record", "pin", "purpose", "csr", "ca", "carrier", "declaration-key", "fraction", "duplicate", "trailing-plan"} {
		t.Run(fault, func(t *testing.T) {
			r, record, pin := bytes.Clone(raw), bytes.Clone(signed), hex.EncodeToString(digest[:])
			switch fault {
			case "signature":
				record[len(record)-1] ^= 1
				d := sha256.Sum256(record)
				pin = hex.EncodeToString(d[:])
			case "bytes":
				record[6] ^= 1
				d := sha256.Sum256(record)
				pin = hex.EncodeToString(d[:])
			case "trailing-record":
				record = append(record, 0)
				d := sha256.Sum256(record)
				pin = hex.EncodeToString(d[:])
			case "pin":
				pin = provisionfixture.PlanIdentifier(99)
			case "purpose":
				r = provisionfixture.SigningDeclaration(t, "node-record", p, now)
			case "csr":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"csr":"request","record":`), 1)
			case "ca":
				r = bytes.Replace(r, []byte(`"record":`), []byte(`"ca":true,"record":`), 1)
			case "carrier":
				r = bytes.ReplaceAll(r, []byte("ardents-carrier-tcp-tls-v2"), []byte("ardents-carrier-quic-v2"))
			case "declaration-key":
				r = bytes.Replace(r, []byte(hex.EncodeToString(key[32:])), []byte(hex.EncodeToString(wrong[32:])), 1)
			case "fraction":
				r = provisionfixture.CertificateDeclaration(t, p, now.Add(time.Nanosecond))
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

func verifyIssuedCertificate(s *networkSigning, der []byte) error {
	return VerifyNodeCertificate(der, epoch.InitialClosedRecord{PublicKey: s.public, ValidFrom: s.from, ValidUntil: s.until})
}
