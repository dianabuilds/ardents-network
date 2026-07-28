package channeldelivery

import (
	"bytes"
	"testing"

	domain "ardents/internal/authority"
	identitycapability "ardents/internal/identity/capability"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"github.com/stretchr/testify/require"
)

func TestCanonicalMemberDeliveryResourcesAreRequestBoundAndBounded(t *testing.T) {
	principal := "p1_local-member"
	target, err := CanonicalizeResource(
		ardentsv1connect.ChannelDeliveryServicePrepareGenerationDeliveryProcedure,
		&protocol.PrepareGenerationDeliveryRequest{Version: 1, SubjectPrincipal: principal},
		"principal",
	)
	require.NoError(t, err)
	require.Equal(t, principal, target.ID)

	binding := &protocol.GenerationDeliveryBinding{
		RealmId:     "r1_00112233445566778899aabbccddeeff",
		OperationId: "rao1_00112233445566778899aabbccddeeff",
		DeliveryId:  "rad1_00112233445566778899aabbccddeeff",
	}
	request := &protocol.InstallGenerationDeliveryRequest{
		Version: 1,
		Sealed:  &protocol.SealedGenerationDelivery{Binding: binding, Envelope: []byte{1}},
	}
	target, err = CanonicalizeResource(
		ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure,
		request, domain.ResourceKindGenerationDelivery,
	)
	require.NoError(t, err)
	expected, valid := domain.GenerationDeliveryResource(
		binding.GetRealmId(), binding.GetOperationId(), binding.GetDeliveryId(),
	)
	require.True(t, valid)
	require.Equal(t, expected, target.ID)

	request.Sealed.Envelope = bytes.Repeat(
		[]byte{1}, identitycapability.MaximumGenerationEnvelopeBytes+1,
	)
	_, err = CanonicalizeResource(
		ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure,
		request, domain.ResourceKindGenerationDelivery,
	)
	require.Error(t, err)

	request.Sealed.Envelope = []byte{1}
	request.Sealed.Binding.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = CanonicalizeResource(
		ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure,
		request, domain.ResourceKindGenerationDelivery,
	)
	require.Error(t, err)
}
