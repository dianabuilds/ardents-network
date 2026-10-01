package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/issuer"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedIssuerProfile retains the public Node configuration name while the
// issuer role owns its local reservation and lifecycle.
type ClosedIssuerProfile = issuer.Profile

func validateClosedIssuerProfile(local ClosedIssuerProfile, source authority.Source, snapshot state.NodeDuty, now time.Time) error {
	return issuer.Validate(local, source, snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func startClosedIssuer(local ClosedIssuerProfile, inputs roleInputs, host closedHostingHandle, snapshot state.NodeDuty) (*dutyHandle, error) {
	if err := validateClosedIssuerProfile(local, inputs.authority, snapshot, inputs.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, inputs.listenOverride)
	if err != nil {
		return nil, err
	}
	role, err := issuer.Start(issuer.Config{Profile: local, Snapshot: snapshot,
		Authority: inputs.authority, CurrentDuty: inputs.currentDuty,
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.ControlAdmissionVerifier(inputs.authority, inputs.now, receiver, host)
		},
		Now: inputs.now, ListenAddress: listen})
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: role.Done, Joined: role.Joined, Protect: func(bool) {}, Usage: role.Usage, Stop: role.Stop, Drain: role.Drain}, nil
}
