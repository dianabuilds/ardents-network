package access

import "context"

type authorizedCallContextKey struct{}

// ContextWithAuthorizedCall stores only calls produced by successful Admit.
// A zero or otherwise unsealed value cannot replace an existing context value.
func ContextWithAuthorizedCall(ctx context.Context, call AuthorizedCall) context.Context {
	if ctx == nil || !call.IsAdmitted() {
		return ctx
	}
	copy := call
	return context.WithValue(ctx, authorizedCallContextKey{}, copy)
}

// AuthorizedCallFromContext returns a copy and rechecks the private admission
// seal so arbitrary context values cannot manufacture Actor or Effective.
func AuthorizedCallFromContext(ctx context.Context) (AuthorizedCall, bool) {
	if ctx == nil {
		return AuthorizedCall{}, false
	}
	call, ok := ctx.Value(authorizedCallContextKey{}).(AuthorizedCall)
	return call, ok && call.IsAdmitted()
}
