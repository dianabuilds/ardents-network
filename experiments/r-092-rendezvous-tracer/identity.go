//go:build ignore

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

const (
	nativeALPN    = "ardents-interactive-route-v1"
	serverDNSName = "r092-rendezvous.invalid"
)

type identitySet struct {
	rootPool                     *x509.CertPool
	server, initiator, responder tls.Certificate
	serverID, initiatorID        [32]byte
	responderID                  [32]byte
}

func deterministicIdentities() (identitySet, error) {
	rootKey := deterministicKey("r092-root")
	rootTemplate := certificateTemplate(1, "R-092 root", true, nil)
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootKey.Public(), rootKey)
	if err != nil {
		return identitySet{}, err
	}
	root, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return identitySet{}, err
	}
	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER})
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(rootPEM) {
		return identitySet{}, errors.New("append synthetic root")
	}
	server, serverID, err := leafIdentity(2, "r092-server", root, rootKey, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	if err != nil {
		return identitySet{}, err
	}
	initiator, initiatorID, err := leafIdentity(3, "r092-initiator", root, rootKey, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	if err != nil {
		return identitySet{}, err
	}
	responder, responderID, err := leafIdentity(4, "r092-responder", root, rootKey, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	if err != nil {
		return identitySet{}, err
	}
	return identitySet{rootPool: pool, server: server, initiator: initiator, responder: responder,
		serverID: serverID, initiatorID: initiatorID, responderID: responderID}, nil
}

func deterministicKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func certificateTemplate(serial int64, commonName string, authority bool, usages []x509.ExtKeyUsage) *x509.Certificate {
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: commonName},
		NotBefore: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2035, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage:  x509.KeyUsageDigitalSignature, ExtKeyUsage: usages, BasicConstraintsValid: true, IsCA: authority}
	if authority {
		template.KeyUsage |= x509.KeyUsageCertSign
	} else if commonName == "r092-server" {
		template.DNSNames = []string{serverDNSName}
	}
	return template
}

func leafIdentity(serial int64, label string, root *x509.Certificate, rootKey ed25519.PrivateKey,
	usages []x509.ExtKeyUsage) (tls.Certificate, [32]byte, error) {
	key := deterministicKey(label)
	template := certificateTemplate(serial, label, false, usages)
	der, err := x509.CreateCertificate(rand.Reader, template, root, key.Public(), rootKey)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, [32]byte{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der, root.Raw}, PrivateKey: key, Leaf: parsed}, publicIdentity(parsed), nil
}

func publicIdentity(certificate *x509.Certificate) [32]byte {
	raw, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil {
		return [32]byte{}
	}
	return sha256.Sum256(raw)
}

func serverTLS(material identitySet) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{material.server}, ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: material.rootPool, NextProtos: []string{nativeALPN}, SessionTicketsDisabled: true}
}

func clientTLS(material identitySet, side string) (*tls.Config, [32]byte, error) {
	certificate, identity := material.initiator, material.initiatorID
	if side == "responder" {
		certificate, identity = material.responder, material.responderID
	} else if side != "initiator" {
		return nil, [32]byte{}, errors.New("client side is unsupported")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, RootCAs: material.rootPool,
		ServerName: serverDNSName, NextProtos: []string{nativeALPN}, SessionTicketsDisabled: true}, identity, nil
}
