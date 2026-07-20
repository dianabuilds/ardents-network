package connectrpc

import (
	"errors"
	"strings"

	identityapi "ardents/internal/identity/api"

	"connectrpc.com/connect"
)

type rpcError struct {
	Code      string
	Category  string
	Message   string
	Domain    string
	Operation string
	Reason    string
	Retryable bool
	Details   map[string]any
}

func toConnectError(err *rpcError) error {
	if err == nil {
		return nil
	}
	code := connect.CodeInternal
	switch err.Category {
	case "invalid_input":
		code = connect.CodeInvalidArgument
	case "not_found":
		code = connect.CodeNotFound
	case "expired":
		code = connect.CodeFailedPrecondition
	case "unverified", "policy_rejected", "forbidden":
		code = connect.CodePermissionDenied
	case "unauthorized":
		code = connect.CodeUnauthenticated
	case "unavailable":
		code = connect.CodeUnavailable
	case "conflict":
		code = connect.CodeAlreadyExists
	case "degraded":
		code = connect.CodeFailedPrecondition
	}
	connectErr := connect.NewError(code, errors.New(err.Message))
	detail, detailErr := connect.NewErrorDetail(toProtoError(*err))
	if detailErr == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
}

func requireRead(call callContext, domain, operation string) *rpcError {
	return requireAccess(call, domain, operation, identityapi.AccessRead)
}

func requireWrite(call callContext, domain, operation string) *rpcError {
	return requireAccess(call, domain, operation, identityapi.AccessWrite)
}

func requireAccess(call callContext, domain, operation string, access identityapi.Access) *rpcError {
	decision := identityapi.AuthorizeAction(call, operation, access)
	if decision.Allowed {
		return nil
	}
	reason := decision.Message
	if decision.Reason != "" {
		reason = decision.Reason
	}
	return &rpcError{
		Code:      decision.Code,
		Category:  errorCategory(decision.Code),
		Message:   decision.Message,
		Domain:    domain,
		Operation: operation,
		Reason:    reason,
		Retryable: false,
		Details:   map[string]any{},
	}
}

func mapAPIError(domain, operation, fallbackCode, message string, retryable bool, err error) *rpcError {
	if err == nil {
		return nil
	}
	code := canonicalCode(fallbackCode, err)
	return &rpcError{
		Code:      code,
		Category:  errorCategory(code),
		Message:   message,
		Domain:    domain,
		Operation: operation,
		Reason:    code,
		Retryable: retryable,
		Details:   map[string]any{},
	}
}

func errorCategory(code string) string {
	switch {
	case strings.Contains(code, "invalid"):
		return "invalid_input"
	case strings.Contains(code, "not_found"):
		return "not_found"
	case strings.Contains(code, "expired"):
		return "expired"
	case strings.Contains(code, "unverified"):
		return "unverified"
	case strings.Contains(code, "policy"):
		return "policy_rejected"
	case strings.Contains(code, "unauthorized"):
		return "unauthorized"
	case strings.Contains(code, "forbidden"):
		return "forbidden"
	case strings.Contains(code, "conflict"):
		return "conflict"
	case strings.Contains(code, "degraded"):
		return "degraded"
	case strings.Contains(code, "unavailable"):
		return "unavailable"
	default:
		return "internal_failure"
	}
}

func canonicalCode(fallback string, err error) string {
	return notFoundAwareCode(conflictAwareCode(policyAwareCode(fallback, err), err), err)
}

func policyAwareCode(fallback string, err error) string {
	if err == nil {
		return fallback
	}
	if strings.Contains(err.Error(), "policy_") {
		return "policy_rejected"
	}
	return fallback
}

func conflictAwareCode(fallback string, err error) string {
	if err == nil {
		return fallback
	}
	text := strings.ToLower(err.Error())
	if strings.Contains(text, "already exists") || strings.Contains(text, "already registered") {
		return "conflict"
	}
	return fallback
}

func notFoundAwareCode(fallback string, err error) string {
	if err == nil {
		return fallback
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return "not_found"
	}
	return fallback
}
