package transfer

import (
	"crypto/ed25519"
	"encoding/base64"
	"time"

	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	discoverysource "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
)

func publishTransferTestNode(disco *discovery.Service, principal, publicKey string, key ed25519.PrivateKey) error {
	now := time.Now().UTC()
	principalID, err := identityprincipal.Parse(principal)
	if err != nil {
		return err
	}
	record := discovery.Record{
		Version:  discoveryrecord.Version,
		Node:     &discoveryrecord.NodeFacts{Principal: principalID, PublicKey: publicKey, Endpoints: []string{"tcp://source"}},
		IssuedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	payload, err := discoveryrecord.Canonical(record)
	if err != nil {
		return err
	}
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	_, err = disco.Import(record, discoverysource.Local)
	return err
}
