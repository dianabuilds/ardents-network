package node

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedControlTokenVerifier authenticates a class-1 or class-3 token and
// reserves the host envelope for the entire admitted control lifetime.
func closedControlTokenVerifier(config runtimeConfig, receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	verify := nodeAuthority(config).TokenVerifier(receiver, config.now)
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		approval, err := verify(input)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		// Reserve the entire class lifetime, including retained Introduction
		// deliveries. A request-size bound does not bound a registration.
		var admitted uint64
		switch input.Class {
		case 1:
			admitted = 64 << 10
		case 3:
			admitted = route.ClosedIntroductionRegistrationByteLimit
		default:
			return route.ClosedAdmissionApproval{}, errors.New("control duty cannot admit forwarding class")
		}
		release, err := hosting.Reserve(config.host, resource.HostingTraffic{Tx: 2 * admitted, Rx: 2 * admitted},
			resource.HostingTraffic{Tx: 16 << 10, Rx: 16 << 10}, input.Deadline)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		approval.Release = release
		return approval, nil
	}
}
