package main

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/planfile"
)

func loadNodeIdentity(plan nodePlan, networkID [32]byte) (node.Config, error) {
	identity, err := node.IdentityKey(plan.IdentityKey)
	if err != nil {
		return node.Config{}, err
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
		Probe: node.ProbeConfig{ListenAddress: plan.Listen, Certificate: certificate, ClientRootPEM: root,
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
