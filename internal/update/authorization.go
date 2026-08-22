package update

import "github.com/dianabuilds/ardents-network/internal/release"

// authorizedRequest resolves the one public authorization at the module
// boundary. The unexported decision field is a package-private test seam only:
// no caller outside this package can construct it, while behavior tests retain
// small synthetic decisions without depending on release's metadata fixtures.
func authorizedRequest(request Request) (Request, bool) {
	if decision, ok := request.Authorization.AcceptedDecision(); ok {
		request.decision = decision
	} else if request.decision.Outcome != release.OutcomeReleaseAccepted {
		return request, false
	}
	if decision, ok := request.RollbackAuthorization.AcceptedDecision(); ok {
		request.rollbackDecision = decision
	}
	return request, true
}
