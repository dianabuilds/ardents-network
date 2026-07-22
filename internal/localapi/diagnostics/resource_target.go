package diagnostics

import (
	"errors"

	diagnosticsapi "ardents/internal/diagnostics"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("diagnostics resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind identityaccess.ResourceKind) (identityaccess.ResourceTarget, error) {
	target := identityaccess.ResourceTarget{Kind: kind}
	valid := true
	switch procedure {
	case ardentsv1connect.DiagnosticsServiceGetDiagnosticsProcedure:
		_, valid = message.(*protocol.GetDiagnosticsRequest)
	case ardentsv1connect.DiagnosticsServiceGetPendingOperationsProcedure:
		_, valid = message.(*protocol.GetPendingOperationsRequest)
	case ardentsv1connect.DiagnosticsServiceGetHealthSummaryProcedure:
		_, valid = message.(*protocol.GetHealthSummaryRequest)
	case ardentsv1connect.DiagnosticsServiceExplainFailureProcedure:
		request, ok := message.(*protocol.ExplainFailureRequest)
		if !ok {
			valid = false
			break
		}
		var err error
		target.ID, err = diagnosticsapi.SubjectAccessResourceID(request.GetScope(), request.GetResourceId())
		valid = err == nil
	case ardentsv1connect.DiagnosticsServiceListRecentEventsProcedure:
		request, ok := message.(*protocol.ListRecentEventsRequest)
		valid = ok && diagnosticsapi.ValidateRecentEventsPage(request.GetLimit(), request.GetCursor()) == nil
	default:
		valid = false
	}
	if !valid {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	return target, nil
}
