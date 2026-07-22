package identity

import (
	"testing"

	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"github.com/stretchr/testify/require"
)

func TestIdentityProcedureCatalogueIsExactAndClosed(t *testing.T) {
	require.Len(t, procedureCatalog, 10)
	for _, procedure := range []string{
		ardentsv1connect.IdentityServiceBeginAuthenticationProcedure,
		ardentsv1connect.IdentityServiceCompleteAuthenticationProcedure,
		ardentsv1connect.IdentityServiceEnrollFirstPrincipalProcedure,
	} {
		require.Equal(t, accessPublicBounded, procedureCatalog[procedure].class)
	}
	for procedure, rule := range procedureCatalog {
		if rule.class == accessProtected {
			require.NotEmpty(t, rule.action, procedure)
		}
	}
	_, known := procedureCatalog["/ardents.v1.IdentityService/Unknown"]
	require.False(t, known)
}

func TestUnknownFieldsAreRejectedRecursively(t *testing.T) {
	top := &protocol.BeginAuthenticationRequest{}
	top.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	require.True(t, hasUnknownFields(top))

	nested := &protocol.IssueAccessGrantRequest{Proposal: &protocol.AccessGrantProposal{Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}}}
	nested.Proposal.Scope.GetNode().ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	require.True(t, hasUnknownFields(nested))
	require.False(t, hasUnknownFields(&protocol.BeginAuthenticationRequest{}))
}
