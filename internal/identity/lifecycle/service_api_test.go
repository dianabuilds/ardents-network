package lifecycle

import (
	"testing"

	identityapi "ardents/internal/identity/api"

	"github.com/stretchr/testify/require"
)

func TestServiceAuthorizeUsesCanonicalSubjectFlow(t *testing.T) {
	svc := New()

	decision := svc.Authorize(identityapi.CallContext{
		Authenticated: true,
		Subject:       identityapi.SubjectRef{Kind: "token", ID: "caller-1"},
		Capabilities:  []string{"node:read"},
	}, "node", identityapi.AccessRead)

	require.True(t, decision.Allowed)
}

func TestServiceAuthorizeRejectsMissingCanonicalSubject(t *testing.T) {
	svc := New()

	decision := svc.Authorize(identityapi.CallContext{
		Authenticated: true,
	}, "node", identityapi.AccessRead)

	require.False(t, decision.Allowed)
	require.Equal(t, "forbidden", decision.Code)
}
