package auth

import (
	"fmt"

	"ardents/internal/identity"

	"connectrpc.com/connect"
)

func Require(call identity.CallContext, operation string, access identity.Access) error {
	decision := identity.AuthorizeAction(call, operation, access)
	if decision.Allowed {
		return nil
	}
	code := connect.CodePermissionDenied
	if !call.Authenticated {
		code = connect.CodeUnauthenticated
	}
	return connect.NewError(code, fmt.Errorf("%s", decision.Message))
}
