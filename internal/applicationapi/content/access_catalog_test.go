package content_test

import (
	applicationadmission "ardents/internal/applicationapi/admission"
	applicationcontent "ardents/internal/applicationapi/content"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"
	"bytes"
	"crypto/ed25519"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestApplicationContentCatalogueCoversEveryProtectedProcedureExactlyOnce(t *testing.T) {
	service := applicationv1.File_api_ardents_application_v1_content_proto.Services().ByName("ContentService")
	require.NotNil(t, service)

	procedures := applicationcontent.ProtectedProcedures()
	require.Len(t, procedures, service.Methods().Len())
	seen := make(map[string]int, len(procedures))
	for _, procedure := range procedures {
		seen[procedure]++
		rule, err := applicationcontent.RuleForProcedure(procedure)
		require.NoError(t, err)
		require.NotEmpty(t, rule.Action)
		require.NotEmpty(t, rule.ResourceKind)
	}
	for index := 0; index < service.Methods().Len(); index++ {
		method := service.Methods().Get(index)
		procedure := "/" + string(service.FullName()) + "/" + string(method.Name())
		require.Equal(t, 1, seen[procedure], "protected procedure %s must be catalogued exactly once", procedure)
	}
}

func TestApplicationContentCatalogueUsesFrozenActionsAndResources(t *testing.T) {
	tests := []struct {
		procedure string
		action    string
		kind      string
	}{
		{applicationv1connect.ContentServicePutProcedure, applicationcontent.ActionPut, "content-owner"},
		{applicationv1connect.ContentServiceGetProcedure, applicationcontent.ActionGet, "owned-content"},
	}
	for _, test := range tests {
		rule, err := applicationcontent.RuleForProcedure(test.procedure)
		require.NoError(t, err)
		require.Equal(t, test.action, rule.Action)
		require.Equal(t, test.kind, rule.ResourceKind)
		require.Equal(t, test.action == applicationcontent.ActionPut, rule.Mutating)
	}

	_, err := applicationcontent.RuleForProcedure("/ardents.application.v1.ContentService/Delete")
	require.ErrorIs(t, err, applicationcontent.ErrUnknownProcedure)
}

func TestApplicationContentRulesComposeAsOneCompleteClosedSet(t *testing.T) {
	contracts, registrations, setErr := applicationcontent.ProtectedProcedureSet()
	require.NoError(t, setErr)

	registry, err := applicationadmission.NewRegistry(contracts, registrations)
	require.NoError(t, err)
	require.Len(t, contracts, 2)
	require.Len(t, registrations, 2)

	_, err = applicationadmission.NewRegistry(contracts, registrations[:1])
	require.Error(t, err)
	_, err = applicationadmission.NewRegistry(contracts, append(registrations, registrations[0]))
	require.Error(t, err)

	for _, contract := range contracts {
		rule, ok := registry.Lookup(contract.Procedure)
		require.True(t, ok)
		require.Equal(t, contract.Action, rule.Action)
		require.Equal(t, contract.ResourceKind, rule.ResourceKind)
		require.Equal(t, contract.OwnerRequired, rule.OwnerRequired)
		require.Equal(t, contract.Mutating, rule.Mutating)
	}
}

func TestApplicationContentRulesOwnResolutionFinalizationAndTargetErrors(t *testing.T) {
	contracts, registrations, setErr := applicationcontent.ProtectedProcedureSet()
	require.NoError(t, setErr)
	registry, err := applicationadmission.NewRegistry(contracts, registrations)
	require.NoError(t, err)

	node := cataloguePrincipal(t, 0x41)
	effective := cataloguePrincipal(t, 0x42)
	audience := identityaccess.Audience{
		Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1,
	}
	tests := []struct {
		procedure string
		request   any
		kind      identityaccess.ResourceKind
		id        string
		mutating  bool
	}{
		{
			procedure: applicationv1connect.ContentServicePutProcedure,
			request:   &applicationv1.PutContentRequest{Payload: []byte("content")},
			kind:      "content-owner",
			mutating:  true,
		},
		{
			procedure: applicationv1connect.ContentServiceGetProcedure,
			request: &applicationv1.GetContentRequest{
				Reference: &applicationv1.ContentReference{Kind: "blob", Id: "bafkreiexact"},
			},
			kind: "owned-content",
			id:   "bafkreiexact",
		},
	}
	for _, test := range tests {
		t.Run(test.procedure, func(t *testing.T) {
			rule, ok := registry.Lookup(test.procedure)
			require.True(t, ok)
			require.Equal(t, test.mutating, rule.Mutating)

			target, resolveErr := rule.Resolve(test.request)
			require.NoError(t, resolveErr)
			require.Equal(t, test.kind, target.Kind)
			require.Equal(t, test.id, target.ID)

			resource, finalizeErr := rule.Finalize(target, audience, effective, effective)
			require.NoError(t, finalizeErr)
			require.Equal(t, effective, resource.Owner.String())
			require.Equal(t, node, resource.Node)
			require.Equal(t, test.kind, resource.Kind)
			require.Equal(t, test.id, resource.ID)
		})
	}

	put, ok := registry.Lookup(applicationv1connect.ContentServicePutProcedure)
	require.True(t, ok)
	mapped := put.MapTargetErr(applicationcontent.ErrPayloadTooLarge)
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(mapped))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, requireApplicationError(t, mapped).GetCode())

	get, ok := registry.Lookup(applicationv1connect.ContentServiceGetProcedure)
	require.True(t, ok)
	_, err = get.Finalize(identityaccess.ResourceTarget{Kind: "node"}, audience, effective, effective)
	require.ErrorIs(t, err, identityaccess.ErrInvalidArgument)
	mapped = get.MapTargetErr(applicationcontent.ErrInvalidResourceTarget)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(mapped))
	require.Equal(t, applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, requireApplicationError(t, mapped).GetCode())
}

func cataloguePrincipal(t *testing.T, marker byte) string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return principal.String()
}
