package testkit

import identityapi "ardents/internal/identity/api"

func AuthorizedCallContext(caller string) identityapi.CallContext {
	return identityapi.CallContext{
		Subject:       identityapi.SubjectRef{Kind: "local", ID: caller},
		Capabilities:  []string{"*"},
		Authenticated: true,
	}
}
