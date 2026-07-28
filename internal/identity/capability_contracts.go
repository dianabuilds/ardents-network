package identity

import (
	"encoding/json"
	"time"
)

type CapabilityPermission uint32

const (
	CapabilitySubscribe CapabilityPermission = 1 << iota
	CapabilityPublish
	CapabilityStoreFetch
	CapabilityFilter
	CapabilityLightpush
	CapabilityDelegate
)

const CapabilityKnownPermissions = CapabilitySubscribe |
	CapabilityPublish |
	CapabilityStoreFetch |
	CapabilityFilter |
	CapabilityLightpush |
	CapabilityDelegate

type CapabilityScope string

const (
	CapabilityRealmDiscovery CapabilityScope = "realm.discovery"
	CapabilityDataExchange   CapabilityScope = "data.exchange"
	CapabilityApplication    CapabilityScope = "channel.application"
	CapabilityControl        CapabilityScope = "realm.capability_control"
)

const (
	ChannelGrantReasonPending    = "channel_grant_pending"
	ChannelGrantReasonRenewalDue = "channel_grant_renewal_due"
	ChannelGrantReasonExpired    = "channel_grant_expired"
	ChannelGrantReasonNotAdopted = "channel_generation_not_adopted"
)

type CapabilitySecret struct {
	raw [32]byte
}

func NewCapabilitySecret(raw []byte) (CapabilitySecret, bool) {
	var secret CapabilitySecret
	if len(raw) != len(secret.raw) {
		return CapabilitySecret{}, false
	}
	copy(secret.raw[:], raw)
	return secret, true
}

func (s CapabilitySecret) Bytes() []byte {
	return append([]byte(nil), s.raw[:]...)
}

func (s CapabilitySecret) Valid() bool {
	var combined byte
	for _, value := range s.raw {
		combined |= value
	}
	return combined != 0
}

func (CapabilitySecret) String() string               { return "[redacted]" }
func (CapabilitySecret) GoString() string             { return "[redacted]" }
func (CapabilitySecret) MarshalJSON() ([]byte, error) { return json.Marshal("[redacted]") }

type CapabilityGrant struct {
	Version          uint32
	ChannelID        [16]byte
	Generation       uint32
	Secret           CapabilitySecret
	GrantID          [16]byte
	IssuerPrincipal  string
	SubjectPrincipal string
	Permissions      CapabilityPermission
	Scope            CapabilityScope
	NotBefore        time.Time
	NotAfter         time.Time
	Signature        []byte
}

func (CapabilityGrant) String() string   { return "capability-grant[redacted]" }
func (CapabilityGrant) GoString() string { return "capability-grant[redacted]" }

func (grant CapabilityGrant) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Version, Generation uint32
		Issuer, Subject     string
		Permissions         CapabilityPermission
		Scope               CapabilityScope
		NotBefore, NotAfter time.Time
	}{
		grant.Version, grant.Generation, grant.IssuerPrincipal, grant.SubjectPrincipal,
		grant.Permissions, grant.Scope, grant.NotBefore, grant.NotAfter,
	})
}

type CapabilityRef string

type CapabilityUse struct {
	Ref        CapabilityRef
	Subject    string
	Permission CapabilityPermission
	Scope      CapabilityScope
	At         time.Time
}

type CapabilitySenderUse struct {
	GrantID    [16]byte
	ChannelID  [16]byte
	Generation uint32
	Subject    string
	Permission CapabilityPermission
	Scope      CapabilityScope
	At         time.Time
	ObservedAt time.Time
}

type ResolvedCapability struct {
	Ref         CapabilityRef
	ChannelID   [16]byte
	Generation  uint32
	GrantID     [16]byte
	Subject     string
	Permissions CapabilityPermission
	Scope       CapabilityScope
	Secret      CapabilitySecret
}

func (ResolvedCapability) String() string   { return "resolved-capability[redacted]" }
func (ResolvedCapability) GoString() string { return "resolved-capability[redacted]" }

func (resolved ResolvedCapability) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Generation  uint32
		Subject     string
		Permissions CapabilityPermission
		Scope       CapabilityScope
	}{
		Generation: resolved.Generation, Subject: resolved.Subject,
		Permissions: resolved.Permissions, Scope: resolved.Scope,
	})
}

type CapabilityRevocation struct {
	Version         uint32
	GrantID         [16]byte
	IssuerPrincipal string
	RevokedAt       time.Time
	Signature       []byte
}

func (CapabilityRevocation) String() string   { return "capability-revocation[redacted]" }
func (CapabilityRevocation) GoString() string { return "capability-revocation[redacted]" }

func (rev CapabilityRevocation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Version   uint32
		Issuer    string
		RevokedAt time.Time
	}{rev.Version, rev.IssuerPrincipal, rev.RevokedAt})
}

type CapabilityDeliveryAttestation struct {
	Version           uint32
	SubjectPrincipal  string
	IdentityPublicKey []byte
	DeliveryPublicKey []byte
	NotBefore         time.Time
	NotAfter          time.Time
	Signature         []byte
}

type CapabilityResolver interface {
	ResolveCapability(CapabilityUse) (ResolvedCapability, error)
}

type CapabilityAdmission interface {
	AllowCapabilityUse(CapabilityUse) error
}

type CapabilitySenderAuthorizer interface {
	AuthorizeCapabilitySender(CapabilitySenderUse) error
}
