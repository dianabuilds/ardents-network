package access

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthorizedCallContextRejectsUnsealedValuesAndReturnsCopy(t *testing.T) {
	base := context.Background()
	require.Equal(t, base, ContextWithAuthorizedCall(base, AuthorizedCall{}))
	_, ok := AuthorizedCallFromContext(base)
	require.False(t, ok)

	call := AuthorizedCall{actor: "actor", effective: "effective", sessionID: "session", seal: &admissionSeal{}}
	ctx := ContextWithAuthorizedCall(base, call)
	stored, ok := AuthorizedCallFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "actor", stored.Actor())
	require.Equal(t, "effective", stored.Effective())

	call.actor = "mutated"
	again, ok := AuthorizedCallFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "actor", again.Actor())
}

func TestAuthorizedCallContextHandlesNilWithoutPanicking(t *testing.T) {
	require.Nil(t, ContextWithAuthorizedCall(nil, AuthorizedCall{}))
	_, ok := AuthorizedCallFromContext(nil)
	require.False(t, ok)
}
