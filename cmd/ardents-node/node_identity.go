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
	root, err := readOperatorInput(plan.ClientRoot, 64<<10)
	if err != nil {
		return node.Config{}, err
	}
	node := node.Config{NetworkID: networkID, IdentityKey: identity,
		LocalRoleStateRoot: plan.LocalRoleStateRoot,
		Probe: node.ProbeConfig{ListenAddress: plan.Listen, Certificate: certificate, ClientRootPEM: root,
			MaximumDuty:  time.Duration(plan.MaximumDutyMS) * time.Millisecond,
			DrainTimeout: time.Duration(plan.DrainTimeoutMS) * time.Millisecond}, PollInterval: 100 * time.Millisecond,
		Quarantine: time.Second, Now: time.Now, ResourceProfile: plan.NodeResourceProfile}
	if err := decodeOperatorFixedHex(plan.NodeID, node.NodeID[:]); err != nil {
		return node, err
	}
	node.Probe.ClientKeyPins, err = decodeOperatorDigests(plan.ClientKeyDigests, 16)
	if err != nil {
		return node, err
	}
	return node, nil
}
