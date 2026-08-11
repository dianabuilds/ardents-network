package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nodelifecycle"
)

func loadNodeIdentity(plan nodePlan, networkID [32]byte) (nodelifecycle.Config, error) {
	identityPEM, err := readNodeFile(plan.IdentityKey, 64<<10)
	if err != nil {
		return nodelifecycle.Config{}, err
	}
	block, _ := pem.Decode(identityPEM)
	if block == nil {
		return nodelifecycle.Config{}, errors.New("node identity key is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nodelifecycle.Config{}, err
	}
	identity, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nodelifecycle.Config{}, errors.New("node identity key is not Ed25519")
	}
	certificate, err := loadNodeKeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return nodelifecycle.Config{}, err
	}
	root, err := readNodeFile(plan.ClientRoot, 64<<10)
	if err != nil {
		return nodelifecycle.Config{}, err
	}
	node := nodelifecycle.Config{NetworkID: networkID, IdentityKey: identity, ListenAddress: plan.Listen,
		Certificate: certificate, ClientRootPEM: root, PollInterval: 100 * time.Millisecond,
		MaximumDuty: 15 * time.Second, DrainTimeout: 15 * time.Second,
		Quarantine: time.Second, Now: time.Now}
	if err := decodeNodeHex(plan.NodeID, node.NodeID[:]); err != nil {
		return node, err
	}
	if len(plan.ClientKeyDigests) == 0 || len(plan.ClientKeyDigests) > 16 {
		return node, errors.New("node client key pin count is invalid")
	}
	for _, encoded := range plan.ClientKeyDigests {
		var pin [32]byte
		if err := decodeNodeHex(encoded, pin[:]); err != nil {
			return node, err
		}
		node.ClientKeyPins = append(node.ClientKeyPins, pin)
	}
	return node, nil
}
