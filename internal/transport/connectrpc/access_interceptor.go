package connectrpc

import (
	"context"
	"fmt"
	"net/http"

	diagapi "ardents/internal/diagnostics/api"
	identityapi "ardents/internal/identity/api"

	"connectrpc.com/connect"
)

type accessInterceptor struct {
	auth  AuthConfig
	audit diagapi.EventWriter
}

func newAccessInterceptor(auth AuthConfig, audit diagapi.EventWriter) *accessInterceptor {
	return &accessInterceptor{auth: auth, audit: audit}
}

func (i *accessInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.authorize(request.Spec().Procedure, request.Header()); err != nil {
			return nil, err
		}
		response, err := next(ctx, request)
		i.recordAuthorizedCommand(request.Spec().Procedure, request.Header())
		return response, err
	}
}

func (i *accessInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *accessInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, connection connect.StreamingHandlerConn) error {
		if err := i.authorize(connection.Spec().Procedure, connection.RequestHeader()); err != nil {
			return err
		}
		return next(ctx, connection)
	}
}

func (i *accessInterceptor) authorize(procedure string, header http.Header) error {
	rule, ok := procedureAccess[procedure]
	if !ok || rule.Action == "" || rule.Domain == "" {
		i.record(rule, "denied", "unknown_procedure", "unverified")
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("operator action is not registered"))
	}
	call, err := i.auth.callContext(header)
	if err != nil {
		i.record(rule, "denied", connectReason(err), "unverified")
		return err
	}
	decision := identityapi.AuthorizeAction(call, rule.Action, rule.Access)
	if !decision.Allowed {
		i.record(rule, "denied", decision.Code, call.Subject.ID)
		return toConnectError(accessRPCError(rule, decision))
	}
	return nil
}

func (i *accessInterceptor) recordAuthorizedCommand(procedure string, header http.Header) {
	rule, ok := procedureAccess[procedure]
	if !ok || rule.Access != identityapi.AccessWrite {
		return
	}
	call, err := i.auth.callContext(header)
	if err != nil {
		return
	}
	i.record(rule, "allowed", "authorized", call.Subject.ID)
}

func accessRPCError(rule accessRule, decision identityapi.Decision) *rpcError {
	return &rpcError{
		Code: decision.Code, Category: errorCategory(decision.Code), Message: decision.Message,
		Domain: rule.Domain, Operation: rule.Action, Reason: decision.Reason,
		Retryable: false, Details: map[string]any{},
	}
}

func connectReason(err error) string {
	if connect.CodeOf(err) == connect.CodeUnauthenticated {
		return "authentication_failed"
	}
	return "binding_or_scope_mismatch"
}

func (i *accessInterceptor) record(rule accessRule, outcome, reason, subject string) {
	if i.audit == nil {
		return
	}
	i.audit.RecordEventCommand(diagapi.RecordEventCommand{
		Domain: "operator_access", Type: "operator_action_" + outcome,
		Resource: rule.Action, Message: "operator action " + outcome, ReasonCode: reason,
		Payload: map[string]any{
			"action": rule.Action, "domain": rule.Domain, "access": string(rule.Access),
			"subject": subject, "outcome": outcome, "reason": reason, "node": i.auth.TargetNode,
		},
	})
}
