package identity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceAuthorizeUsesCanonicalSubjectFlow(t *testing.T) {
	svc := NewService()

	decision := svc.Authorize(CallContext{
		Authenticated: true,
		Subject:       SubjectRef{Kind: "token", ID: "caller-1"},
		Capabilities:  []string{"node:read"},
	}, "node", AccessRead)

	require.True(t, decision.Allowed)
}

func TestServiceAuthorizeRejectsMissingCanonicalSubject(t *testing.T) {
	svc := NewService()

	decision := svc.Authorize(CallContext{
		Authenticated: true,
	}, "node", AccessRead)

	require.False(t, decision.Allowed)
	require.Equal(t, "forbidden", decision.Code)
}
