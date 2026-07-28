package channeldelivery

import (
	"errors"

	domain "ardents/internal/authority"
	identityaccess "ardents/internal/identity/access"
	identitycapability "ardents/internal/identity/capability"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("channel delivery resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind string) (identityaccess.ResourceTarget, error) {
	switch procedure {
	case ardentsv1connect.ChannelDeliveryServicePrepareGenerationDeliveryProcedure:
		request, ok := message.(*protocol.PrepareGenerationDeliveryRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			request.GetSubjectPrincipal() == "" {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: request.GetSubjectPrincipal(),
		}, nil
	case ardentsv1connect.ChannelDeliveryServiceInstallGenerationDeliveryProcedure:
		request, ok := message.(*protocol.InstallGenerationDeliveryRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			request.GetSealed() == nil || request.GetSealed().GetBinding() == nil ||
			len(request.GetSealed().ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetSealed().GetBinding().ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetSealed().GetEnvelope()) == 0 ||
			len(request.GetSealed().GetEnvelope()) > identitycapability.MaximumGenerationEnvelopeBytes {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		binding := request.GetSealed().GetBinding()
		resource, valid := domain.GenerationDeliveryResource(
			binding.GetRealmId(), binding.GetOperationId(), binding.GetDeliveryId(),
		)
		if !valid {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: resource,
		}, nil
	default:
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
}
