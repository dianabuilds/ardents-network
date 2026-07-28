package capability

import (
	"fmt"
	"time"

	identityapi "ardents/internal/identity"
)

type persistedGrant struct {
	Version          uint32                           `json:"version"`
	ChannelID        [16]byte                         `json:"channel_id"`
	Generation       uint32                           `json:"generation"`
	Secret           []byte                           `json:"secret"`
	GrantID          [16]byte                         `json:"grant_id"`
	IssuerPrincipal  string                           `json:"issuer_principal"`
	SubjectPrincipal string                           `json:"subject_principal"`
	Permissions      identityapi.CapabilityPermission `json:"permissions"`
	Scope            identityapi.CapabilityScope      `json:"scope"`
	NotBefore        time.Time                        `json:"not_before"`
	NotAfter         time.Time                        `json:"not_after"`
	Signature        []byte                           `json:"signature"`
}

type persistedRevocation struct {
	Version         uint32    `json:"version"`
	GrantID         [16]byte  `json:"grant_id"`
	IssuerPrincipal string    `json:"issuer_principal"`
	RevokedAt       time.Time `json:"revoked_at"`
	Signature       []byte    `json:"signature"`
}

type persistedDeliveryReceipt struct {
	Version            uint32                      `json:"version"`
	RealmID            string                      `json:"realm_id"`
	AuthorityPrincipal string                      `json:"authority_principal"`
	AuthorityEpoch     uint64                      `json:"authority_epoch"`
	OperationID        string                      `json:"operation_id"`
	DeliveryID         string                      `json:"delivery_id"`
	EnvelopeDigest     string                      `json:"envelope_digest"`
	AuthoritySequence  uint64                      `json:"authority_sequence"`
	ChannelID          [16]byte                    `json:"channel_id"`
	ChannelClass       identityapi.CapabilityScope `json:"channel_class"`
	Generation         uint32                      `json:"generation"`
	RecipientPrincipal string                      `json:"recipient_principal"`
	DeliveryKeyDigest  string                      `json:"delivery_key_digest"`
	Phase              string                      `json:"phase"`
	CreatedAt          time.Time                   `json:"created_at"`
	ExpiresAt          time.Time                   `json:"expires_at"`
	MAC                []byte                      `json:"mac"`
}

func persistGrant(grant identityapi.CapabilityGrant) persistedGrant {
	return persistedGrant{
		Version: grant.Version, ChannelID: grant.ChannelID,
		Generation: grant.Generation, Secret: grant.Secret.Bytes(), GrantID: grant.GrantID,
		IssuerPrincipal: grant.IssuerPrincipal, SubjectPrincipal: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope,
		NotBefore: grant.NotBefore, NotAfter: grant.NotAfter,
		Signature: append([]byte(nil), grant.Signature...),
	}
}

func (stored persistedGrant) restore() (identityapi.CapabilityGrant, error) {
	secret, ok := identityapi.NewCapabilitySecret(stored.Secret)
	if !ok {
		return identityapi.CapabilityGrant{}, fmt.Errorf("persisted capability secret is invalid")
	}
	return identityapi.CapabilityGrant{
		Version: stored.Version, ChannelID: stored.ChannelID,
		Generation: stored.Generation, Secret: secret, GrantID: stored.GrantID,
		IssuerPrincipal: stored.IssuerPrincipal, SubjectPrincipal: stored.SubjectPrincipal,
		Permissions: stored.Permissions, Scope: stored.Scope,
		NotBefore: stored.NotBefore, NotAfter: stored.NotAfter,
		Signature: append([]byte(nil), stored.Signature...),
	}, nil
}

func persistRevocation(rev identityapi.CapabilityRevocation) persistedRevocation {
	return persistedRevocation{
		Version: rev.Version, GrantID: rev.GrantID,
		IssuerPrincipal: rev.IssuerPrincipal, RevokedAt: rev.RevokedAt,
		Signature: append([]byte(nil), rev.Signature...),
	}
}

func (stored persistedRevocation) restore() identityapi.CapabilityRevocation {
	return identityapi.CapabilityRevocation{
		Version: stored.Version, GrantID: stored.GrantID,
		IssuerPrincipal: stored.IssuerPrincipal, RevokedAt: stored.RevokedAt,
		Signature: append([]byte(nil), stored.Signature...),
	}
}

func persistDeliveryReceipt(
	receipt GenerationDeliveryReceipt,
	expiresAt time.Time,
) persistedDeliveryReceipt {
	return persistedDeliveryReceipt{
		Version: receipt.Version, RealmID: receipt.RealmID,
		AuthorityPrincipal: receipt.AuthorityPrincipal, AuthorityEpoch: receipt.AuthorityEpoch,
		OperationID: receipt.OperationID, DeliveryID: receipt.DeliveryID,
		EnvelopeDigest: receipt.EnvelopeDigest, AuthoritySequence: receipt.AuthoritySequence,
		ChannelID: receipt.ChannelID, ChannelClass: receipt.ChannelClass,
		Generation: receipt.Generation, RecipientPrincipal: receipt.RecipientPrincipal,
		DeliveryKeyDigest: receipt.DeliveryKeyDigest, Phase: receipt.Phase,
		CreatedAt: receipt.CreatedAt, ExpiresAt: expiresAt,
		MAC: append([]byte(nil), receipt.MAC...),
	}
}

func (stored persistedDeliveryReceipt) restore() GenerationDeliveryReceipt {
	return GenerationDeliveryReceipt{
		Version: stored.Version, RealmID: stored.RealmID,
		AuthorityPrincipal: stored.AuthorityPrincipal, AuthorityEpoch: stored.AuthorityEpoch,
		OperationID: stored.OperationID, DeliveryID: stored.DeliveryID,
		EnvelopeDigest: stored.EnvelopeDigest, AuthoritySequence: stored.AuthoritySequence,
		ChannelID: stored.ChannelID, ChannelClass: stored.ChannelClass,
		Generation: stored.Generation, RecipientPrincipal: stored.RecipientPrincipal,
		DeliveryKeyDigest: stored.DeliveryKeyDigest, Phase: stored.Phase,
		CreatedAt: stored.CreatedAt, MAC: append([]byte(nil), stored.MAC...),
	}
}
