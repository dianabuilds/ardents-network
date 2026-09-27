package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/introduction"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ClosedIntroductionProfile retains the Node configuration name while the
// Introduction role owns its listener, slot state and spend root.
type ClosedIntroductionProfile = introduction.Profile

func validateClosedIntroductionProfile(local ClosedIntroductionProfile, config runtimeConfig, snapshot state.NodeDuty, now time.Time) error {
	return introduction.Validate(local, nodeAuthority(config), snapshot, now, literalNodeEndpoint(snapshot.ProbeEndpoint))
}

func newClosedIntroductionServer(config runtimeConfig, snapshot state.NodeDuty) (*introduction.Server, error) {
	if err := validateClosedIntroductionProfile(config.ClosedIntroduction, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	listen, err := closedListenAddress(snapshot.ProbeEndpoint, config.ClosedListenOverride)
	if err != nil {
		return nil, err
	}
	return introduction.Start(introduction.Config{Profile: config.ClosedIntroduction, Snapshot: snapshot,
		Authority: nodeAuthority(config), CurrentDuty: func() (state.NodeDuty, error) { return currentFacts(config) },
		VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
			return closedControlTokenVerifier(config, receiver)
		},
		Now: config.now, ListenAddress: listen})
}

func startClosedIntroduction(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	server, err := newClosedIntroductionServer(config, snapshot)
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: server.Done(), Joined: server.Joined(), Protect: func(bool) {}, Usage: server.Usage,
		Stop: func() { _ = server.Stop() }, Drain: func(ctx context.Context) error {
			return server.Drain(ctx, config.ClosedIntroduction.DrainTimeout)
		}}, nil
}
