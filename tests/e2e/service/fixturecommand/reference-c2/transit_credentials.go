//go:build referencec2

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (value transitCredential) valid() bool {
	return value.Grant != "" && value.Certificate != "" && value.PrivateKey != ""
}

func (input config) entryInvite() ([]byte, error) {
	raw, err := base64.RawStdEncoding.DecodeString(input.Invite)
	if err != nil || len(raw) == 0 {
		return nil, errors.New("C2 fixture Entry Invite encoding is invalid")
	}
	return raw, nil
}

// decode returns one fixture-local opaque grant, the corresponding private
// TLS identity, and the public grant tuple used only to index Endpoint-local
// provisioning. This package is built only by the process e2e test.
func (value transitCredential) decode() ([]byte, tls.Certificate, route.TransitGrant, error) {
	if !value.valid() {
		return nil, tls.Certificate{}, route.TransitGrant{}, errors.New("C2 fixture transit credential is unavailable")
	}
	raw, err := base64.RawStdEncoding.DecodeString(value.Grant)
	if err != nil {
		return nil, tls.Certificate{}, route.TransitGrant{}, errors.New("C2 fixture transit grant encoding is invalid")
	}
	grant, err := route.DecodeTransitGrant(raw)
	if err != nil {
		return nil, tls.Certificate{}, route.TransitGrant{}, err
	}
	certificate, err := tls.X509KeyPair([]byte(value.Certificate), []byte(value.PrivateKey))
	if err != nil || len(certificate.Certificate) != 1 {
		return nil, tls.Certificate{}, route.TransitGrant{}, errors.New("C2 fixture transit client key pair is invalid")
	}
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, tls.Certificate{}, route.TransitGrant{}, errors.New("C2 fixture transit client certificate is invalid")
	}
	digest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil || digest != grant.ClientKeyDigest {
		return nil, tls.Certificate{}, route.TransitGrant{}, errors.New("C2 fixture transit grant does not match its client key")
	}
	return raw, certificate, grant, nil
}
