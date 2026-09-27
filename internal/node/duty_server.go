package node

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

const nativeRouteUnavailableReason = "native Route assignment is not implemented"

func startDuty(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if snapshot.Profile == carrier.ClosedRouteProfile {
		if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeIssuer, config.now()); available {
			return startClosedIssuer(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeForwarding, config.now()); available {
			return startClosedForwarding(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeReachability, config.now()); available {
			return startClosedResolution(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeIntroduction, config.now()); available {
			return startClosedIntroduction(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, ardp.PurposeDataJoin, config.now()); available {
			return startClosedDataJoin(config, snapshot)
		}
		return nil, errors.New("closed Route assignment is not locally implemented")
	}
	if snapshot.Profile == route.Profile {
		return nil, errors.New(nativeRouteUnavailableReason)
	}
	selected, err := config.probe.Start(newProbeDuty(snapshot))
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: selected.Done, Protect: selected.Protect, Usage: selected.Usage,
		Stop: selected.Stop, Drain: selected.Drain}, nil
}
