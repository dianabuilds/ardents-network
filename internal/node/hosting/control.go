package hosting

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// ControlAdmissionVerifier authenticates a class-1 or class-3 token and
// reserves the host envelope for the entire admitted control lifetime.
func ControlAdmissionVerifier(source authority.Source, now func() time.Time, receiver route.ClosedRoleReceiver, host Host) route.ClosedAdmissionVerifier {
	verify := source.TokenVerifier(receiver, now)
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
			admitted = admission.ControlClass.ByteLimit()
		case 3:
			admitted = admission.RegistrationClass.ByteLimit()
		default:
			return route.ClosedAdmissionApproval{}, errors.New("control duty cannot admit forwarding class")
		}
		release, err := Reserve(host, hostingbudget.Traffic{Tx: 2 * admitted, Rx: 2 * admitted},
			hostingbudget.Traffic{Tx: 16 << 10, Rx: 16 << 10}, input.Deadline)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		approval.Release = release
		return approval, nil
	}
}
