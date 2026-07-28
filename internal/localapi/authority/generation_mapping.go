package authority

import (
	"fmt"

	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func attestationFromWire(value *protocol.GenerationDeliveryAttestation) (identityapi.CapabilityDeliveryAttestation, error) {
	if value == nil || value.GetNotBefore() == nil || value.GetNotAfter() == nil ||
		value.GetNotBefore().CheckValid() != nil || value.GetNotAfter().CheckValid() != nil {
		return identityapi.CapabilityDeliveryAttestation{}, fmt.Errorf("delivery attestation is invalid")
	}
	return identityapi.CapabilityDeliveryAttestation{
		Version: value.GetVersion(), SubjectPrincipal: value.GetSubjectPrincipal(),
		IdentityPublicKey: append([]byte(nil), value.GetIdentityPublicKey()...),
		DeliveryPublicKey: append([]byte(nil), value.GetDeliveryPublicKey()...),
		NotBefore:         rpc.Time(value.GetNotBefore()), NotAfter: rpc.Time(value.GetNotAfter()),
		Signature: append([]byte(nil), value.GetSignature()...),
	}, nil
}

func sealedToWire(value identitycapability.SealedGenerationDelivery) *protocol.SealedGenerationDelivery {
	return &protocol.SealedGenerationDelivery{
		Binding: &protocol.GenerationDeliveryBinding{
			Version: value.Binding.Version, RealmId: value.Binding.RealmID,
			AuthorityPrincipal: value.Binding.AuthorityPrincipal,
			AuthorityEpoch:     value.Binding.AuthorityEpoch, AuthoritySequence: value.Binding.AuthoritySequence,
			OperationId: value.Binding.OperationID, DeliveryId: value.Binding.DeliveryID,
			ChannelId:    value.Binding.ChannelID[:],
			ChannelClass: string(value.Binding.ChannelClass), Generation: value.Binding.Generation,
			RecipientPrincipal: value.Binding.RecipientPrincipal,
			DeliveryKeyDigest:  value.Binding.DeliveryKeyDigest,
			ExpiresAt:          rpc.Timestamp(value.Binding.ExpiresAt),
		},
		Envelope:       append([]byte(nil), value.Envelope...),
		EnvelopeDigest: value.EnvelopeDigest,
	}
}

func receiptFromWire(value *protocol.GenerationDeliveryReceipt) (identitycapability.GenerationDeliveryReceipt, error) {
	if value == nil || value.GetCreatedAt() == nil || value.GetCreatedAt().CheckValid() != nil {
		return identitycapability.GenerationDeliveryReceipt{}, fmt.Errorf("delivery receipt is invalid")
	}
	channelID, err := fixedID(value.GetChannelId())
	if err != nil {
		return identitycapability.GenerationDeliveryReceipt{}, err
	}
	return identitycapability.GenerationDeliveryReceipt{
		Version: value.GetVersion(), RealmID: value.GetRealmId(),
		AuthorityPrincipal: value.GetAuthorityPrincipal(), AuthorityEpoch: value.GetAuthorityEpoch(),
		OperationID: value.GetOperationId(), DeliveryID: value.GetDeliveryId(),
		EnvelopeDigest: value.GetEnvelopeDigest(), AuthoritySequence: value.GetAuthoritySequence(),
		ChannelID: channelID, ChannelClass: identityapi.CapabilityScope(value.GetChannelClass()),
		Generation:         value.GetGeneration(),
		RecipientPrincipal: value.GetRecipientPrincipal(), Phase: value.GetPhase(),
		DeliveryKeyDigest: value.GetDeliveryKeyDigest(),
		CreatedAt:         rpc.Time(value.GetCreatedAt()), MAC: append([]byte(nil), value.GetMac()...),
	}, nil
}

func fixedID(value []byte) ([16]byte, error) {
	if len(value) != 16 {
		return [16]byte{}, fmt.Errorf("channel id is invalid")
	}
	var result [16]byte
	copy(result[:], value)
	return result, nil
}

func activationToWire(
	value identitycapability.GenerationActivation,
) *protocol.GenerationActivation {
	return &protocol.GenerationActivation{
		Version: value.Version, RealmId: value.RealmID,
		AuthorityPrincipal: value.AuthorityPrincipal,
		AuthorityEpoch:     value.AuthorityEpoch, AuthoritySequence: value.AuthoritySequence,
		OperationId: value.OperationID, ChannelId: value.ChannelID[:],
		ChannelClass:       string(value.ChannelClass),
		PreviousGeneration: value.PreviousGeneration, Generation: value.Generation,
		EffectiveAt:      rpc.Timestamp(value.EffectiveAt),
		DrainDeadline:    rpc.Timestamp(value.DrainDeadline),
		CheckpointDigest: value.CheckpointDigest,
		Signature:        append([]byte(nil), value.Signature...),
	}
}
