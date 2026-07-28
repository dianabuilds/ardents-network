// Package authority adapts the protected Operator protocol to the Realm
// Authority domain. It does not own authentication, product policy, or state.
package authority

import (
	"errors"

	domain "ardents/internal/authority"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
)

var ErrInvalidResourceTarget = errors.New("authority resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind string) (identityaccess.ResourceTarget, error) {
	switch procedure {
	case ardentsv1connect.AuthorityServiceCreateRealmAuthorityProcedure:
		request, ok := message.(*protocol.CreateRealmAuthorityRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: domain.PrimaryAuthorityInstance,
		}, nil
	case ardentsv1connect.AuthorityServiceInspectRealmAuthorityProcedure:
		request, ok := message.(*protocol.InspectRealmAuthorityRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			!domain.ValidRealmID(request.GetRealmId()) {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: request.GetRealmId(),
		}, nil
	case ardentsv1connect.AuthorityServiceInspectChannelProcedure:
		request, ok := message.(*protocol.InspectChannelRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			!domain.ValidRealmID(request.GetRealmId()) ||
			len(request.GetChannelId()) != 16 {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		var channelID [16]byte
		copy(channelID[:], request.GetChannelId())
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.ChannelResource(request.GetRealmId(), channelID),
		}, nil
	case ardentsv1connect.AuthorityServiceIssueInitialGenerationProcedure:
		request, ok := message.(*protocol.IssueInitialGenerationRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			request.GetRecipientAttestation() == nil ||
			len(request.GetRecipientAttestation().ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) == 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes ||
			!domain.ValidRealmID(request.GetRealmId()) {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.InitialGenerationDeliveryResource(request.GetRealmId(), request.GetRequestId()),
		}, nil
	case ardentsv1connect.AuthorityServiceAcknowledgeInitialGenerationProcedure:
		request, ok := message.(*protocol.AcknowledgeInitialGenerationRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 || request.GetReceipt() == nil ||
			len(request.GetReceipt().ProtoReflect().GetUnknown()) != 0 {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		resource, valid := domain.GenerationDeliveryResource(
			request.GetRealmId(), request.GetOperationId(), request.GetReceipt().GetDeliveryId(),
		)
		if !valid {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind), ID: resource,
		}, nil
	case ardentsv1connect.AuthorityServiceRotateChannelProcedure:
		request, ok := message.(*protocol.RotateChannelRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) == 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes ||
			!domain.ValidRealmID(request.GetRealmId()) ||
			len(request.GetChannelId()) != 16 ||
			len(request.GetRecipientAttestations()) == 0 ||
			len(request.GetRecipientAttestations()) > domain.MaxMembersPerChannel {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		for _, attestation := range request.GetRecipientAttestations() {
			if attestation == nil ||
				len(attestation.ProtoReflect().GetUnknown()) != 0 {
				return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
			}
		}
		var channelID [16]byte
		copy(channelID[:], request.GetChannelId())
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.ChannelResource(request.GetRealmId(), channelID),
		}, nil
	case ardentsv1connect.AuthorityServiceRenewChannelGrantsProcedure:
		request, ok := message.(*protocol.RenewChannelGrantsRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) == 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes ||
			!domain.ValidRealmID(request.GetRealmId()) ||
			len(request.GetChannelId()) != 16 ||
			len(request.GetRecipientAttestations()) == 0 ||
			len(request.GetRecipientAttestations()) > domain.MaxMembersPerChannel {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		for _, attestation := range request.GetRecipientAttestations() {
			if attestation == nil ||
				len(attestation.ProtoReflect().GetUnknown()) != 0 {
				return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
			}
		}
		var channelID [16]byte
		copy(channelID[:], request.GetChannelId())
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.ChannelResource(request.GetRealmId(), channelID),
		}, nil
	case ardentsv1connect.AuthorityServiceChangeChannelMembershipProcedure:
		request, ok := message.(*protocol.ChangeChannelMembershipRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetRequestId()) == 0 ||
			len(request.GetRequestId()) > domain.MaxRequestIDBytes ||
			!domain.ValidRealmID(request.GetRealmId()) ||
			len(request.GetChannelId()) != 16 ||
			len(request.GetRecipientAttestations()) == 0 ||
			len(request.GetRecipientAttestations()) > domain.MaxMembersPerChannel {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		for _, attestation := range request.GetRecipientAttestations() {
			if attestation == nil || len(attestation.ProtoReflect().GetUnknown()) != 0 {
				return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
			}
		}
		var channelID [16]byte
		copy(channelID[:], request.GetChannelId())
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.ChannelResource(request.GetRealmId(), channelID),
		}, nil
	case ardentsv1connect.AuthorityServiceSubmitDeploymentFenceEvidenceProcedure:
		request, ok := message.(*protocol.SubmitDeploymentFenceEvidenceRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			!domain.ValidRealmID(request.GetRealmId()) ||
			len(request.GetChannelId()) != 16 || request.GetEvidence() == nil ||
			len(request.GetEvidence().ProtoReflect().GetUnknown()) != 0 ||
			len(request.GetEvidence().GetControls()) == 0 ||
			len(request.GetEvidence().GetControls()) > domain.MaxDeploymentFenceControls {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		for _, control := range request.GetEvidence().GetControls() {
			if control == nil || len(control.ProtoReflect().GetUnknown()) != 0 {
				return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
			}
		}
		var channelID [16]byte
		copy(channelID[:], request.GetChannelId())
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   domain.ChannelResource(request.GetRealmId(), channelID),
		}, nil
	case ardentsv1connect.AuthorityServiceCommitChannelActivationProcedure:
		request, ok := message.(*protocol.CommitChannelActivationRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		resource := domain.OperationResource(request.GetRealmId(), request.GetOperationId())
		if !domain.ValidOperationResource(resource) {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		return identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(kind),
			ID:   resource,
		}, nil
	case ardentsv1connect.AuthorityServiceAcknowledgeChannelActivationProcedure:
		request, ok := message.(*protocol.AcknowledgeChannelActivationRequest)
		if !ok || len(request.ProtoReflect().GetUnknown()) != 0 ||
			request.GetReceipt() == nil ||
			len(request.GetReceipt().ProtoReflect().GetUnknown()) != 0 {
			return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
		}
		resource, valid := domain.GenerationDeliveryResource(
			request.GetRealmId(), request.GetOperationId(),
			request.GetReceipt().GetDeliveryId(),
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
