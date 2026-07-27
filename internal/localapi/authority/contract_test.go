package authority

import (
	"testing"

	identitycontract "ardents/api/ardents/identity/v1"
	domain "ardents/internal/authority"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestAuthorityProceduresHaveExactDirectOperatorContracts(t *testing.T) {
	create, ok := localauth.RuleForProcedure(ardentsv1connect.AuthorityServiceCreateRealmAuthorityProcedure)
	require.True(t, ok)
	require.Equal(t, domain.ActionCreate, create.Action)
	require.Equal(t, domain.ResourceKindAuthorityInstance, create.ResourceKind)
	require.True(t, create.Mutating)

	inspect, ok := localauth.RuleForProcedure(ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure)
	require.True(t, ok)
	require.Equal(t, domain.ActionInspect, inspect.Action)
	require.Equal(t, domain.ResourceKindRealm, inspect.ResourceKind)
	require.False(t, inspect.Mutating)

	require.True(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, domain.ActionCreate))
	require.True(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, domain.ActionInspect))
	require.False(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, domain.ActionCreate))
	require.False(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, domain.ActionInspect))
}

func TestCanonicalAuthorityResourcesAreExactAndBounded(t *testing.T) {
	target, err := CanonicalizeResource(
		ardentsv1connect.AuthorityServiceCreateRealmAuthorityProcedure,
		&protocol.CreateRealmAuthorityRequest{
			Version: domain.ContractVersion, RequestId: "request-001",
			RealmClass: domain.RealmClassProduction,
		},
		domain.ResourceKindAuthorityInstance,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ResourceKindAuthorityInstance, string(target.Kind))
	require.Equal(t, domain.PrimaryAuthorityInstance, target.ID)

	realmID := "r1_00112233445566778899aabbccddeeff"
	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure,
		&protocol.InspectRealmAuthorityRequest{Version: domain.ContractVersion, RealmId: realmID},
		domain.ResourceKindRealm,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ResourceKindRealm, string(target.Kind))
	require.Equal(t, realmID, target.ID)

	unknown := &protocol.InspectRealmAuthorityRequest{Version: domain.ContractVersion, RealmId: realmID}
	unknown.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure,
		unknown, domain.ResourceKindRealm,
	)
	require.Error(t, err)

	_, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure,
		&protocol.InspectRealmAuthorityRequest{Version: domain.ContractVersion, RealmId: "r1_bad"},
		domain.ResourceKindRealm,
	)
	require.Error(t, err)
}

func TestApplicationProtocolHasNoAuthorityService(t *testing.T) {
	_, err := protoregistry.GlobalFiles.FindDescriptorByName(
		protoreflect.FullName("ardents.application.v1.AuthorityService"),
	)
	require.ErrorIs(t, err, protoregistry.NotFound)
}
