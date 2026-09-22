package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const nativeRouteUnavailableReason = "native Route assignment is not implemented"

func startDuty(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	if snapshot.Profile == route.ClosedRouteProfile {
		if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeIssuer, config.now()); available {
			return startClosedIssuer(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeForwarding, config.now()); available {
			return startClosedForwarding(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeReachability, config.now()); available {
			return startClosedResolution(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeIntroduction, config.now()); available {
			return startClosedIntroduction(config, snapshot)
		}
		if _, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeDataJoin, config.now()); available {
			return startClosedDataJoin(config, snapshot)
		}
		return nil, errors.New("closed Route assignment is not locally implemented")
	}
	if snapshot.Profile == route.Profile {
		return nil, errors.New(nativeRouteUnavailableReason)
	}
	return config.probe.startProbe(newProbeDuty(snapshot))
}
