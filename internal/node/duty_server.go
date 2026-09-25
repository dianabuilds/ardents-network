package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

const nativeRouteUnavailableReason = "native Route assignment is not implemented"

func startDuty(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	if snapshot.Profile == route.ClosedRouteProfile {
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
	return config.probe.startProbe(newProbeDuty(snapshot))
}
