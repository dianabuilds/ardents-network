package main

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/resource"
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
	config := node.Config{HostingRoot: plan.HostingRoot, ClosedListenOverride: plan.ClosedListenOverride, NetworkID: networkID, IdentityKey: identity,
		LocalRoleStateRoot: plan.LocalRoleStateRoot,
		PollInterval:       100 * time.Millisecond,
		Quarantine:         time.Second, Now: time.Now, ResourceProfile: plan.NodeResourceProfile}
	if err := decodeOperatorFixedHex(plan.NodeID, config.NodeID[:]); err != nil {
		return config, err
	}
	if plan.ClosedIssuer != nil {
		config.ClosedIssuer = node.ClosedIssuerProfile{Root: plan.ClosedIssuer.Root, AdmissionRoot: plan.ClosedIssuer.AdmissionRoot, Certificate: certificate,
			ConnectionLimit: plan.ClosedIssuer.ConnectionLimit, DrainTimeout: time.Duration(plan.ClosedIssuer.DrainTimeoutMS) * time.Millisecond}
	}
	if plan.ClosedForwarding != nil {
		config.ClosedForwarding = node.ClosedForwardingProfile{Root: plan.ClosedForwarding.Root, Certificate: certificate,
			ConnectionLimit: plan.ClosedForwarding.ConnectionLimit, DrainTimeout: time.Duration(plan.ClosedForwarding.DrainTimeoutMS) * time.Millisecond,
			HostingRoot:          plan.ClosedForwarding.HostingRoot,
			CarrierRelayEndpoint: plan.ClosedForwarding.CarrierRelayEndpoint,
			AdmissionTraffic:     resource.HostingTraffic{Tx: plan.ClosedForwarding.AdmissionTraffic.Tx, Rx: plan.ClosedForwarding.AdmissionTraffic.Rx},
			TerminationTraffic:   resource.HostingTraffic{Tx: plan.ClosedForwarding.TerminationTraffic.Tx, Rx: plan.ClosedForwarding.TerminationTraffic.Rx}}
	}
	if plan.ClosedResolution != nil {
		config.ClosedResolution = node.ClosedResolutionProfile{Root: plan.ClosedResolution.Root, AdmissionRoot: plan.ClosedResolution.AdmissionRoot,
			Certificate: certificate, ConnectionLimit: plan.ClosedResolution.ConnectionLimit, DrainTimeout: time.Duration(plan.ClosedResolution.DrainTimeoutMS) * time.Millisecond}
	}
	if plan.ClosedIntroduction != nil {
		config.ClosedIntroduction = node.ClosedIntroductionProfile{AdmissionRoot: plan.ClosedIntroduction.AdmissionRoot,
			Certificate: certificate, ConnectionLimit: plan.ClosedIntroduction.ConnectionLimit, DrainTimeout: time.Duration(plan.ClosedIntroduction.DrainTimeoutMS) * time.Millisecond}
	}
	if plan.ClosedDataJoin != nil {
		config.ClosedDataJoin = node.ClosedDataJoinProfile{HostingRoot: plan.ClosedDataJoin.HostingRoot, AdmissionRoot: plan.ClosedDataJoin.AdmissionRoot,
			Certificate: certificate, ConnectionLimit: plan.ClosedDataJoin.ConnectionLimit, DrainTimeout: time.Duration(plan.ClosedDataJoin.DrainTimeoutMS) * time.Millisecond}
	}
	if plan.Rendezvous != nil || plan.Initiator != nil || plan.Introduction != nil || plan.Responder != nil || plan.TransitIssuer != nil || plan.ClosedIssuer != nil || plan.ClosedForwarding != nil || plan.ClosedResolution != nil || plan.ClosedIntroduction != nil || plan.ClosedDataJoin != nil {
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
