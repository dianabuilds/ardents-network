package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/resolution"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedResolutionProfile keeps the process configuration surface stable.
type ClosedResolutionProfile = resolution.ClosedResolutionProfile

func validateClosedResolutionProfile(local ClosedResolutionProfile, config runtimeConfig, snapshot state.NodeDuty, now time.Time) error {
	return resolution.Validate(local, nodeAuthority(config), snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedResolution(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedResolutionProfile(config.ClosedResolution, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, config.ClosedListenOverride)
	if err != nil {
		return nil, err
	}
	role, err := resolution.Start(resolution.Config{Profile: config.ClosedResolution, Snapshot: snapshot,
		Authority: nodeAuthority(config), CurrentDuty: func() (state.NodeDuty, error) { return currentFacts(config) },
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return closedControlTokenVerifier(config, receiver)
		},
		Now: config.now, ListenAddress: listen})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Joined: role.Joined, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}
