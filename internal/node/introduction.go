package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/introduction"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedIntroductionProfile retains the Node configuration name while the
// Introduction role owns its listener, slot state and spend root.
type ClosedIntroductionProfile = introduction.Profile

func validateClosedIntroductionProfile(local ClosedIntroductionProfile, source authority.Source, snapshot state.NodeDuty, now time.Time) error {
	return introduction.Validate(local, source, snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func newClosedIntroductionServer(local ClosedIntroductionProfile, inputs roleInputs, snapshot state.NodeDuty) (*introduction.Server, error) {
	if err := validateClosedIntroductionProfile(local, inputs.authority, snapshot, inputs.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, inputs.listenOverride)
	if err != nil {
		return nil, err
	}
	return introduction.Start(introduction.Config{Profile: local, Snapshot: snapshot,
		Authority: inputs.authority, CurrentDuty: inputs.currentDuty,
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return hosting.ControlAdmissionVerifier(inputs.authority, inputs.now, receiver, inputs.host)
		},
		Now: inputs.now, ListenAddress: listen})
}

func startClosedIntroduction(local ClosedIntroductionProfile, inputs roleInputs, snapshot state.NodeDuty) (*dutyHandle, error) {
	server, err := newClosedIntroductionServer(local, inputs, snapshot)
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: server.Done(), Joined: server.Joined(), Protect: func(bool) {}, Usage: server.Usage,
		Stop: func() { _ = server.Stop() }, Drain: func(ctx context.Context) error {
			return server.Drain(ctx, local.DrainTimeout)
		}}, nil
}
