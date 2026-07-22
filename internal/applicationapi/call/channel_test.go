package call

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelRejectsZeroInvalidAndForeignAdmissions(t *testing.T) {
	principal := PrincipalFacts{
		Actor: "p1_actor", Effective: "p1_actor", Node: "p1_node", Interface: 2, ProtocolMajor: 1,
		Action: "application.content.get", ResourceNode: "p1_node", ResourceOwner: "p1_actor",
		ResourceKind: "owned-content", ResourceID: "blob",
	}
	injector, extractor := NewChannel()
	foreignInjector, _ := NewChannel()

	_, ok := extractor.Extract(context.Background())
	require.False(t, ok)
	_, ok = extractor.Extract(foreignInjector.WithPrincipal(context.Background(), principal))
	require.False(t, ok)
	_, ok = extractor.Extract(injector.WithPrincipal(context.Background(), PrincipalFacts{}))
	require.False(t, ok)

	ctx := injector.WithPrincipal(context.Background(), principal)
	stored, ok := extractor.Extract(ctx)
	require.True(t, ok)
	require.True(t, stored.IsPrincipal())
	require.Equal(t, principal.Actor, stored.Actor())
	require.Equal(t, stored.Actor(), stored.Effective())
}
