package main

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
)

func loadNodeIdentity(plan nodePlan, networkID [32]byte) (node.Config, error) {
	identity, err := node.IdentityKey(plan.IdentityKey)
	if err != nil {
		return node.Config{}, err
	}
	certificate, err := readOperatorKeyPair(plan.ServerCertificate, plan.ServerKey)
	if err != nil {
		return node.Config{}, err
	}
	config := node.Config{NetworkID: networkID, IdentityKey: identity,
		LocalRoleStateRoot: plan.LocalRoleStateRoot,
		PollInterval:       100 * time.Millisecond,
		Quarantine:         time.Second, Now: time.Now, ResourceProfile: plan.NodeResourceProfile}
	if err := decodeOperatorFixedHex(plan.NodeID, config.NodeID[:]); err != nil {
		return config, err
	}
	if plan.Rendezvous != nil {
		config.Rendezvous = node.RendezvousProfile{Certificate: certificate, HandshakeLimit: plan.Rendezvous.HandshakeLimit,
			WaitingLimit: plan.Rendezvous.WaitingLimit, PairLimit: plan.Rendezvous.PairLimit,
			PairByteLimit: plan.Rendezvous.PairByteLimit, DrainTimeout: time.Duration(plan.Rendezvous.DrainTimeoutMS) * time.Millisecond}
		return config, nil
	}
	root, err := readOperatorInput(plan.ClientRoot, 64<<10)
	if err != nil {
		return config, err
	}
	config.Probe = node.ProbeConfig{ListenAddress: plan.Listen, Certificate: certificate, ClientRootPEM: root,
		MaximumDuty: time.Duration(plan.MaximumDutyMS) * time.Millisecond, DrainTimeout: time.Duration(plan.DrainTimeoutMS) * time.Millisecond}
	config.Probe.ClientKeyPins, err = decodeOperatorDigests(plan.ClientKeyDigests, 16)
	if err != nil {
		return config, err
	}
	return config, nil
}
