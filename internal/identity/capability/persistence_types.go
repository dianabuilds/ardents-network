package capability

import (
	"fmt"
	"time"

	identityapi "ardents/internal/identity/api"
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
