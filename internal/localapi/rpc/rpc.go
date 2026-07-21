// Package rpc owns shared Connect context, error details, and deadline mechanics.
// It does not own authorization or product mapping.
package rpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ardents/internal/identity"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type CallContext = identity.CallContext

type Error struct {
	Code, Category, Message, Domain, Operation, Reason string
	Retryable                                          bool
	Details                                            map[string]any
}

func Respond[T any](auth localauth.Config, header http.Header, invoke func(CallContext) (*T, *Error)) (*connect.Response[T], error) {
	call, err := auth.CallContext(header)
	if err != nil {
		return nil, err
	}
	message, rpcErr := invoke(call)
	if rpcErr != nil {
		return nil, ToConnectError(rpcErr)
	}
	return connect.NewResponse(message), nil
}

func RequireRead(call CallContext, domain, operation string) *Error {
	return requireAccess(call, domain, operation, identity.AccessRead)
}

func RequireWrite(call CallContext, domain, operation string) *Error {
	return requireAccess(call, domain, operation, identity.AccessWrite)
}

func requireAccess(call CallContext, domain, operation string, access identity.Access) *Error {
	decision := identity.AuthorizeAction(call, operation, access)
	if decision.Allowed {
		return nil
	}
	return &Error{Code: decision.Code, Category: ErrorCategory(decision.Code), Message: decision.Message, Domain: domain, Operation: operation, Reason: decision.Reason}
}

func MapError(domain, operation, fallback, message string, retryable bool, err error) *Error {
	if err == nil {
		return nil
	}
	code := CanonicalCode(fallback, err)
	return &Error{Code: code, Category: ErrorCategory(code), Message: message, Domain: domain, Operation: operation, Reason: code, Retryable: retryable, Details: map[string]any{}}
}

func NotFound(domain, operation, message string) *Error {
	return &Error{Code: "not_found", Category: "not_found", Message: message, Domain: domain, Operation: operation, Reason: "not_found", Details: map[string]any{}}
}

func ToConnectError(err *Error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeInternal
	switch err.Category {
	case "invalid_input":
		code = connect.CodeInvalidArgument
	case "not_found":
		code = connect.CodeNotFound
	case "expired", "degraded":
		code = connect.CodeFailedPrecondition
	case "unverified", "policy_rejected", "forbidden":
		code = connect.CodePermissionDenied
	case "unauthorized":
		code = connect.CodeUnauthenticated
	case "unavailable":
		code = connect.CodeUnavailable
	case "conflict":
		code = connect.CodeAlreadyExists
	}
	result := connect.NewError(code, errors.New(err.Message))
	detail, detailErr := connect.NewErrorDetail(&protocol.Error{Code: err.Code, Category: err.Category, Message: err.Message, Domain: err.Domain, Retryable: err.Retryable, Operation: err.Operation, Reason: err.Reason})
	if detailErr == nil {
		result.AddDetail(detail)
	}
	return result
}

func CanonicalCode(fallback string, err error) string {
	if err == nil {
		return fallback
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "not found"):
		return "not_found"
	case strings.Contains(text, "already exists"), strings.Contains(text, "already registered"):
		return "conflict"
	case strings.Contains(text, "policy_"):
		return "policy_rejected"
	default:
		return fallback
	}
}

func ErrorCategory(code string) string {
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

func MutationContext(parent context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(parent)
	if deadline, ok := parent.Deadline(); ok {
		return context.WithDeadline(base, deadline)
	}
	return base, func() {}
}

func Timestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func TimestampPointer(value *time.Time) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}
	return timestamppb.New(*value)
}

func Time(value *timestamppb.Timestamp) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.AsTime().UTC()
}

func Struct(value map[string]any) *structpb.Struct {
	if len(value) == 0 {
		return nil
	}
	result, err := structpb.NewStruct(value)
	if err != nil {
		return nil
	}
	return result
}

func Map(value *structpb.Struct) map[string]any {
	if value == nil {
		return nil
	}
	return value.AsMap()
}
