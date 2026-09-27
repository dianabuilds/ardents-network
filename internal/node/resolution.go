package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/resolution"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedResolutionProfile keeps the process configuration surface stable.
type ClosedResolutionProfile = resolution.ClosedResolutionProfile

func validateClosedResolutionProfile(local ClosedResolutionProfile, source authority.Source, snapshot state.NodeDuty, now time.Time) error {
	return resolution.Validate(local, source, snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedResolution(local ClosedResolutionProfile, inputs roleInputs, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedResolutionProfile(local, inputs.authority, snapshot, inputs.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, inputs.listenOverride)
	if err != nil {
		return nil, err
	}
	role, err := resolution.Start(resolution.Config{Profile: local, Snapshot: snapshot,
		Authority: inputs.authority, CurrentDuty: inputs.currentDuty,
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.ControlAdmissionVerifier(inputs.authority, inputs.now, receiver, inputs.host)
		},
		Now: inputs.now, ListenAddress: listen})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Joined: role.Joined, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}
