package subject

import (
	"testing"

	identityapi "ardents/internal/identity/api"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCallUsesCanonicalFieldsFirst(t *testing.T) {
	call := identityapi.CallContext{
		Authenticated: true,
		Subject:       identityapi.SubjectRef{Kind: "token", ID: "subject-1"},
		Capabilities:  []string{"node:read"},
	}

	subject := NormalizeCall(call)
	require.Equal(t, identityapi.SubjectRef{Kind: "token", ID: "subject-1"}, subject.Ref)
	require.Equal(t, []string{"node:read"}, subject.Capabilities)
}

func TestNormalizeCallRequiresCanonicalFields(t *testing.T) {
	subject := NormalizeCall(identityapi.CallContext{
		Authenticated: true,
	})

	require.Equal(t, identityapi.SubjectRef{}, subject.Ref)
	require.Empty(t, subject.Capabilities)
}
