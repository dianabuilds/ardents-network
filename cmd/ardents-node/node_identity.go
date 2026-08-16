package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/node/probe"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func loadNodeIdentity(plan nodePlan, networkID [32]byte) (node.Config, error) {
	identityPEM, err := planfile.Read(plan.IdentityKey, 64<<10)
	if err != nil {
		return node.Config{}, err
	}
	block, _ := pem.Decode(identityPEM)
	if block == nil {
		return node.Config{}, errors.New("node identity key is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return node.Config{}, err
	}
	identity, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return node.Config{}, errors.New("node identity key is not Ed25519")
	}
	certificate, err := planfile.KeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return node.Config{}, err
	}
	root, err := planfile.Read(plan.ClientRoot, 64<<10)
	if err != nil {
		return node.Config{}, err
	}
	node := node.Config{NetworkID: networkID, IdentityKey: identity,
		LocalRoleStateRoot: plan.LocalRoleStateRoot,
		Probe: probe.Config{ListenAddress: plan.Listen, Certificate: certificate, ClientRootPEM: root,
			MaximumDuty:  time.Duration(plan.MaximumDutyMS) * time.Millisecond,
			DrainTimeout: time.Duration(plan.DrainTimeoutMS) * time.Millisecond}, PollInterval: 100 * time.Millisecond,
		Quarantine: time.Second, Now: time.Now, ResourceProfile: plan.NodeResourceProfile}
	if err := planfile.FixedHex(plan.NodeID, node.NodeID[:]); err != nil {
		return node, err
	}
	node.Probe.ClientKeyPins, err = planfile.Digests(plan.ClientKeyDigests, 16)
	if err != nil {
		return node, err
	}
	return node, nil
}
