package identity

import (
	"testing"

	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"github.com/stretchr/testify/require"
)

func TestIdentityProcedureCatalogueIsExactAndClosed(t *testing.T) {
	require.Len(t, procedureCatalog, 12)
	service := protocol.File_api_ardents_v1_identity_proto.Services().ByName("IdentityService")
	require.NotNil(t, service)
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
		_, known := procedureCatalog[procedure]
		require.True(t, known, procedure)
	}
	for _, procedure := range []string{
		ardentsv1connect.IdentityServiceBeginAuthenticationProcedure,
		ardentsv1connect.IdentityServiceCompleteAuthenticationProcedure,
		ardentsv1connect.IdentityServiceEnrollFirstPrincipalProcedure,
		ardentsv1connect.IdentityServiceImportDelegationRevocationProcedure,
	} {
		require.Equal(t, accessPublicBounded, procedureCatalog[procedure].class)
	}
	for procedure, rule := range procedureCatalog {
		if rule.class == accessProtected {
			require.NotEmpty(t, rule.action, procedure)
			require.NotEmpty(t, rule.resourceKind, procedure)
		}
		exported, known := RuleForProcedure(procedure)
		require.True(t, known, procedure)
		require.Equal(t, rule.action, exported.Action, procedure)
		require.Equal(t, rule.resourceKind, exported.ResourceKind, procedure)
		require.Equal(t, rule.mutating, exported.Mutating, procedure)
	}
	require.Equal(t, accessSessionLifecycle, procedureCatalog[ardentsv1connect.IdentityServiceEndSessionProcedure].class)
	_, known := procedureCatalog["/ardents.v1.IdentityService/Unknown"]
	require.False(t, known)
	_, known = RuleForProcedure("/ardents.v1.IdentityService/Unknown")
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
