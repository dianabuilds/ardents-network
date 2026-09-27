package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/issuer"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func validateClosedIssuerProfile(local ClosedIssuerProfile, config runtimeConfig, snapshot state.NodeDuty, now time.Time) error {
	return issuer.Validate(local, nodeAuthority(config), snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedIssuer(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedIssuerProfile(config.ClosedIssuer, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, config.ClosedListenOverride)
	if err != nil {
		return nil, err
	}
	role, err := issuer.Start(issuer.Config{Profile: config.ClosedIssuer, Snapshot: snapshot,
		Authority: nodeAuthority(config), CurrentDuty: func() (state.NodeDuty, error) { return currentFacts(config) },
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.ControlAdmissionVerifier(nodeAuthority(config), config.now, receiver, config.host)
		},
		Now: config.now, ListenAddress: listen})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Joined: role.Joined, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}
