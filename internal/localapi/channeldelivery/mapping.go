package channeldelivery

import (
	"fmt"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func attestationToWire(value identityapi.CapabilityDeliveryAttestation) *protocol.GenerationDeliveryAttestation {
	return &protocol.GenerationDeliveryAttestation{
		Version: value.Version, SubjectPrincipal: value.SubjectPrincipal,
		IdentityPublicKey: append([]byte(nil), value.IdentityPublicKey...),
		DeliveryPublicKey: append([]byte(nil), value.DeliveryPublicKey...),
		NotBefore:         rpc.Timestamp(value.NotBefore), NotAfter: rpc.Timestamp(value.NotAfter),
		Signature: append([]byte(nil), value.Signature...),
	}
}

func sealedFromWire(value *protocol.SealedGenerationDelivery) (identitycapability.SealedGenerationDelivery, error) {
	if value == nil || value.GetBinding() == nil || value.GetBinding().GetExpiresAt() == nil ||
		value.GetBinding().GetExpiresAt().CheckValid() != nil {
		return identitycapability.SealedGenerationDelivery{}, fmt.Errorf("sealed delivery is invalid")
	}
	channelID, err := fixedID(value.GetBinding().GetChannelId())
	if err != nil {
		return identitycapability.SealedGenerationDelivery{}, err
	}
	binding := value.GetBinding()
	return identitycapability.SealedGenerationDelivery{
		Binding: identitycapability.GenerationDeliveryBinding{
			Version: binding.GetVersion(), RealmID: binding.GetRealmId(),
			AuthorityPrincipal: binding.GetAuthorityPrincipal(),
			AuthorityEpoch:     binding.GetAuthorityEpoch(), AuthoritySequence: binding.GetAuthoritySequence(),
			OperationID: binding.GetOperationId(), DeliveryID: binding.GetDeliveryId(), ChannelID: channelID,
			ChannelClass: identityapi.CapabilityScope(binding.GetChannelClass()),
			Generation:   binding.GetGeneration(), RecipientPrincipal: binding.GetRecipientPrincipal(),
			DeliveryKeyDigest: binding.GetDeliveryKeyDigest(),
			ExpiresAt:         rpc.Time(binding.GetExpiresAt()),
		},
		Envelope:       append([]byte(nil), value.GetEnvelope()...),
		EnvelopeDigest: value.GetEnvelopeDigest(),
	}, nil
}

func receiptToWire(value identitycapability.GenerationDeliveryReceipt) *protocol.GenerationDeliveryReceipt {
	return &protocol.GenerationDeliveryReceipt{
		Version: value.Version, RealmId: value.RealmID,
		AuthorityPrincipal: value.AuthorityPrincipal, AuthorityEpoch: value.AuthorityEpoch,
		OperationId: value.OperationID, DeliveryId: value.DeliveryID,
		EnvelopeDigest: value.EnvelopeDigest, AuthoritySequence: value.AuthoritySequence,
		ChannelId: value.ChannelID[:], ChannelClass: string(value.ChannelClass),
		Generation:         value.Generation,
		RecipientPrincipal: value.RecipientPrincipal, Phase: value.Phase,
		DeliveryKeyDigest: value.DeliveryKeyDigest,
		CreatedAt:         rpc.Timestamp(value.CreatedAt), Mac: append([]byte(nil), value.MAC...),
	}
}

func fixedID(value []byte) ([16]byte, error) {
	if len(value) != 16 {
		return [16]byte{}, fmt.Errorf("channel id is invalid")
	}
	var result [16]byte
	copy(result[:], value)
	return result, nil
}
