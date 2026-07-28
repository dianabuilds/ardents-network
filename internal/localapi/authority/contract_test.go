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
	inspectChannel, ok := localauth.RuleForProcedure(
		ardentsv1connect.AuthorityServiceInspectChannelProcedure,
	)
	require.True(t, ok)
	require.Equal(t, domain.ActionInspect, inspectChannel.Action)
	require.Equal(t, domain.ResourceKindChannel, inspectChannel.ResourceKind)
	require.False(t, inspectChannel.Mutating)

	require.True(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, domain.ActionCreate))
	require.True(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, domain.ActionInspect))
	require.False(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, domain.ActionCreate))
	require.False(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, domain.ActionInspect))

	for procedure, action := range map[string]string{
		ardentsv1connect.AuthorityServiceIssueInitialGenerationProcedure:          domain.ActionIssueDelivery,
		ardentsv1connect.AuthorityServiceAcknowledgeInitialGenerationProcedure:    domain.ActionAcknowledgeDelivery,
		ardentsv1connect.AuthorityServiceRotateChannelProcedure:                   domain.ActionRotateGeneration,
		ardentsv1connect.AuthorityServiceRenewChannelGrantsProcedure:              domain.ActionRotateGeneration,
		ardentsv1connect.AuthorityServiceCommitChannelActivationProcedure:         domain.ActionCommitActivation,
		ardentsv1connect.AuthorityServiceAcknowledgeChannelActivationProcedure:    domain.ActionAcknowledgeActivation,
		ardentsv1connect.AuthorityServiceChangeChannelMembershipProcedure:         domain.ActionChangeMembership,
		ardentsv1connect.AuthorityServiceSubmitDeploymentFenceEvidenceProcedure:   domain.ActionChangeMembership,
		ardentsv1connect.ChannelDeliveryServicePrepareGenerationDeliveryProcedure: "realm.channel.delivery.prepare",
		ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure: "realm.channel.delivery.install",
		ardentsv1connect.ChannelDeliveryServiceActivateGenerationProcedure:        "realm.channel.generation.activate",
	} {
		rule, registered := localauth.RuleForProcedure(procedure)
		require.True(t, registered, procedure)
		require.Equal(t, action, rule.Action)
		require.True(t, rule.Mutating)
		require.True(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, action))
		require.False(t, identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, action))
	}
}

func TestCanonicalRotationResourcesAreExactAndBounded(t *testing.T) {
	realmID := "r1_00112233445566778899aabbccddeeff"
	operationID := "rao1_00112233445566778899aabbccddeeff"
	deliveryID := "rad1_00112233445566778899aabbccddeeff"
	channelID := []byte("0123456789abcdef")

	target, err := CanonicalizeResource(
		ardentsv1connect.AuthorityServiceRotateChannelProcedure,
		&protocol.RotateChannelRequest{
			Version: 1, RequestId: "rotation-001", RealmId: realmID,
			ChannelId:             channelID,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{{}},
		},
		domain.ResourceKindChannel,
	)
	require.NoError(t, err)
	var channel [16]byte
	copy(channel[:], channelID)
	require.Equal(t, domain.ChannelResource(realmID, channel), target.ID)

	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceRenewChannelGrantsProcedure,
		&protocol.RenewChannelGrantsRequest{
			Version: 1, RequestId: "renewal-001", RealmId: realmID,
			ChannelId:             channelID,
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{{}},
		},
		domain.ResourceKindChannel,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ChannelResource(realmID, channel), target.ID)

	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceChangeChannelMembershipProcedure,
		&protocol.ChangeChannelMembershipRequest{
			Version: 1, RequestId: "membership-001", RealmId: realmID,
			ChannelId: channelID, Change: string(domain.MembershipChangeAdd),
			TargetPrincipal:       "p1_target",
			RecipientAttestations: []*protocol.GenerationDeliveryAttestation{{}},
		},
		domain.ResourceKindChannel,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ChannelResource(realmID, channel), target.ID)

	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceSubmitDeploymentFenceEvidenceProcedure,
		&protocol.SubmitDeploymentFenceEvidenceRequest{
			Version: 1, RealmId: realmID, OperationId: operationID,
			ChannelId: channelID,
			Evidence: &protocol.DeploymentFenceEvidence{
				Controls: []*protocol.DeploymentFenceControl{{}},
			},
		},
		domain.ResourceKindChannel,
	)
	require.NoError(t, err)
	require.Equal(t, domain.ChannelResource(realmID, channel), target.ID)

	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceCommitChannelActivationProcedure,
		&protocol.CommitChannelActivationRequest{
			Version: 1, RealmId: realmID, OperationId: operationID,
		},
		domain.ResourceKindOperation,
	)
	require.NoError(t, err)
	require.Equal(t, domain.OperationResource(realmID, operationID), target.ID)

	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceAcknowledgeChannelActivationProcedure,
		&protocol.AcknowledgeChannelActivationRequest{
			Version: 1, RealmId: realmID, OperationId: operationID,
			Receipt: &protocol.GenerationDeliveryReceipt{DeliveryId: deliveryID},
		},
		domain.ResourceKindGenerationDelivery,
	)
	require.NoError(t, err)
	expected, valid := domain.GenerationDeliveryResource(realmID, operationID, deliveryID)
	require.True(t, valid)
	require.Equal(t, expected, target.ID)

	unknown := &protocol.GenerationDeliveryAttestation{}
	unknown.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceRotateChannelProcedure,
		&protocol.RotateChannelRequest{
			Version: 1, RequestId: "rotation-001", RealmId: realmID,
			ChannelId: channelID, RecipientAttestations: []*protocol.GenerationDeliveryAttestation{unknown},
		},
		domain.ResourceKindChannel,
	)
	require.Error(t, err)
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

	channelID := []byte("0123456789abcdef")
	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceInspectChannelProcedure,
		&protocol.InspectChannelRequest{
			Version: domain.ContractVersion,
			RealmId: "r1_00112233445566778899aabbccddeeff", ChannelId: channelID,
		},
		domain.ResourceKindChannel,
	)
	require.NoError(t, err)

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
	_, err = protoregistry.GlobalFiles.FindDescriptorByName(
		protoreflect.FullName("ardents.application.v1.ChannelDeliveryService"),
	)
	require.ErrorIs(t, err, protoregistry.NotFound)
}

func TestCanonicalInitialGenerationResourcesAreExactAndRejectNestedUnknownFields(t *testing.T) {
	realmID := "r1_00112233445566778899aabbccddeeff"
	requestID := "delivery-001"
	target, err := CanonicalizeResource(
		ardentsv1connect.AuthorityServiceIssueInitialGenerationProcedure,
		&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID, RequestId: requestID,
			RecipientAttestation: &protocol.GenerationDeliveryAttestation{},
		},
		domain.ResourceKindGenerationDelivery,
	)
	require.NoError(t, err)
	require.Equal(t, domain.InitialGenerationDeliveryResource(realmID, requestID), target.ID)

	operationID := "rao1_00112233445566778899aabbccddeeff"
	deliveryID := "rad1_00112233445566778899aabbccddeeff"
	target, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceAcknowledgeInitialGenerationProcedure,
		&protocol.AcknowledgeInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID, OperationId: operationID,
			Receipt: &protocol.GenerationDeliveryReceipt{DeliveryId: deliveryID},
		},
		domain.ResourceKindGenerationDelivery,
	)
	require.NoError(t, err)
	expected, valid := domain.GenerationDeliveryResource(realmID, operationID, deliveryID)
	require.True(t, valid)
	require.Equal(t, expected, target.ID)

	unknown := &protocol.GenerationDeliveryAttestation{}
	unknown.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = CanonicalizeResource(
		ardentsv1connect.AuthorityServiceIssueInitialGenerationProcedure,
		&protocol.IssueInitialGenerationRequest{
			Version: domain.ContractVersion, RealmId: realmID, RequestId: requestID,
			RecipientAttestation: unknown,
		},
		domain.ResourceKindGenerationDelivery,
	)
	require.Error(t, err)
}
