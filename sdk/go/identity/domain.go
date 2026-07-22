package identity

import (
	"bytes"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Interface string

const (
	InterfaceOperator    Interface = "operator"
	InterfaceApplication Interface = "application"
)

type Audience struct {
	Node          string
	Interface     Interface
	ProtocolMajor uint32
}
type ScopeKind string

const (
	ScopeNode           ScopeKind = "node"
	ScopePrincipalOwned ScopeKind = "principal-owned"
	ScopeExact          ScopeKind = "exact"
)

type ResourceRef struct{ Node, Owner, Kind, CanonicalID string }
type ResourceScope struct {
	Kind     ScopeKind
	Owner    string
	Resource ResourceRef
}
type CredentialPurpose string

const PurposeAuthenticate CredentialPurpose = "authenticate"

type KeyCredentialSpec struct {
	Subject             string
	RootPublicKey       []byte
	DeviceID            string
	DevicePublicKey     []byte
	Purposes            []CredentialPurpose
	NotBefore, NotAfter time.Time
}
type KeyCredential struct{ KeyCredentialSpec }
type AccessGrant struct {
	Issuer, Subject     string
	Audience            Audience
	Actions             []string
	Scope               ResourceScope
	NotBefore, NotAfter time.Time
}
type DelegationSpec struct {
	Delegator, Delegatee string
	Audience             Audience
	Actions              []string
	Scope                ResourceScope
	NotBefore, NotAfter  time.Time
	Credential           *Artifact
}
type Delegation struct {
	Delegator, Delegatee string
	Audience             Audience
	Actions              []string
	Scope                ResourceScope
	NotBefore, NotAfter  time.Time
	CredentialID         string
}
type DeviceRevocation struct {
	TargetID, Issuer, TargetDeviceID, Subject string
	Audience                                  Audience
	RevokedAt                                 time.Time
}
type AccessGrantRevocation struct {
	TargetID, Issuer string
	Audience         Audience
	RevokedAt        time.Time
}
type DelegationRevocation struct {
	TargetID, Issuer, Delegator, Delegatee, CredentialID string
	Audience                                             Audience
	RevokedAt                                            time.Time
}

func (a *Artifact) KeyCredential() *KeyCredential {
	p, ok := a.payload.(*identityv1.KeyCredentialPayload)
	if !ok {
		return nil
	}
	return &KeyCredential{KeyCredentialSpec: credentialFromProto(p)}
}
func (a *Artifact) AccessGrant() *AccessGrant {
	p, ok := a.payload.(*identityv1.AccessGrantPayload)
	if !ok {
		return nil
	}
	return &AccessGrant{Issuer: p.Issuer, Subject: p.Subject, Audience: audienceFromProto(p.Audience), Actions: append([]string(nil), p.Actions...), Scope: scopeFromProto(p.Scope), NotBefore: p.NotBefore.AsTime(), NotAfter: p.NotAfter.AsTime()}
}
func (a *Artifact) Delegation() *Delegation {
	p, ok := a.payload.(*identityv1.DelegationPayload)
	if !ok {
		return nil
	}
	return &Delegation{Delegator: p.Delegator, Delegatee: p.Delegatee, Audience: audienceFromProto(p.Audience), Actions: append([]string(nil), p.Actions...), Scope: scopeFromProto(p.Scope), NotBefore: p.NotBefore.AsTime(), NotAfter: p.NotAfter.AsTime(), CredentialID: p.GetCredential().GetId()}
}
func (a *Artifact) DeviceRevocation() *DeviceRevocation {
	p, ok := a.payload.(*identityv1.DeviceRevocationPayload)
	if !ok {
		return nil
	}
	return &DeviceRevocation{TargetID: p.TargetId, Issuer: p.Issuer, TargetDeviceID: p.TargetDeviceId, Subject: p.Subject, Audience: audienceFromProto(p.Audience), RevokedAt: p.RevokedAt.AsTime()}
}
func (a *Artifact) AccessGrantRevocation() *AccessGrantRevocation {
	p, ok := a.payload.(*identityv1.AccessGrantRevocationPayload)
	if !ok {
		return nil
	}
	return &AccessGrantRevocation{TargetID: p.TargetId, Issuer: p.Issuer, Audience: audienceFromProto(p.Audience), RevokedAt: p.RevokedAt.AsTime()}
}
func (a *Artifact) DelegationRevocation() *DelegationRevocation {
	p, ok := a.payload.(*identityv1.DelegationRevocationPayload)
	if !ok {
		return nil
	}
	return &DelegationRevocation{TargetID: p.TargetId, Issuer: p.Issuer, Delegator: p.Delegator, Delegatee: p.Delegatee, CredentialID: p.GetCredential().GetId(), Audience: audienceFromProto(p.Audience), RevokedAt: p.RevokedAt.AsTime()}
}

func credentialFromProto(p *identityv1.KeyCredentialPayload) KeyCredentialSpec {
	purposes := make([]CredentialPurpose, len(p.Purposes))
	for i := range p.Purposes {
		if p.Purposes[i] == identityv1.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE {
			purposes[i] = PurposeAuthenticate
		}
	}
	return KeyCredentialSpec{Subject: p.Subject, RootPublicKey: bytes.Clone(p.RootPublicKey), DeviceID: p.DeviceId, DevicePublicKey: bytes.Clone(p.DevicePublicKey), Purposes: purposes, NotBefore: p.NotBefore.AsTime(), NotAfter: p.NotAfter.AsTime()}
}
func credentialToProto(s KeyCredentialSpec) *identityv1.KeyCredentialPayload {
	purposes := make([]identityv1.CredentialPurpose, len(s.Purposes))
	for i, p := range s.Purposes {
		if p == PurposeAuthenticate {
			purposes[i] = identityv1.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE
		}
	}
	return &identityv1.KeyCredentialPayload{Version: identitycontract.Version, Subject: s.Subject, RootPublicKey: bytes.Clone(s.RootPublicKey), DeviceId: s.DeviceID, DevicePublicKey: bytes.Clone(s.DevicePublicKey), Purposes: purposes, NotBefore: timestamp(s.NotBefore), NotAfter: timestamp(s.NotAfter)}
}
func audienceToProto(a Audience) *identityv1.Audience {
	var i identityv1.Interface
	switch a.Interface {
	case InterfaceOperator:
		i = identityv1.Interface_INTERFACE_OPERATOR
	case InterfaceApplication:
		i = identityv1.Interface_INTERFACE_APPLICATION
	}
	return &identityv1.Audience{Node: a.Node, Interface: i, ProtocolMajor: a.ProtocolMajor}
}
func audienceFromProto(a *identityv1.Audience) Audience {
	if a == nil {
		return Audience{}
	}
	var i Interface
	if a.Interface == identityv1.Interface_INTERFACE_OPERATOR {
		i = InterfaceOperator
	} else if a.Interface == identityv1.Interface_INTERFACE_APPLICATION {
		i = InterfaceApplication
	}
	return Audience{Node: a.Node, Interface: i, ProtocolMajor: a.ProtocolMajor}
}
func scopeToProto(s ResourceScope) *identityv1.ResourceScope {
	switch s.Kind {
	case ScopeNode:
		return &identityv1.ResourceScope{Scope: &identityv1.ResourceScope_Node{Node: &identityv1.NodeScope{}}}
	case ScopePrincipalOwned:
		return &identityv1.ResourceScope{Scope: &identityv1.ResourceScope_PrincipalOwned{PrincipalOwned: &identityv1.PrincipalOwnedScope{Owner: s.Owner}}}
	case ScopeExact:
		return &identityv1.ResourceScope{Scope: &identityv1.ResourceScope_Exact{Exact: &identityv1.ExactScope{Resource: &identityv1.ResourceRef{Node: s.Resource.Node, Owner: s.Resource.Owner, Kind: s.Resource.Kind, CanonicalId: s.Resource.CanonicalID}}}}
	}
	return &identityv1.ResourceScope{}
}
func scopeFromProto(s *identityv1.ResourceScope) ResourceScope {
	if s == nil {
		return ResourceScope{}
	}
	switch x := s.Scope.(type) {
	case *identityv1.ResourceScope_Node:
		return ResourceScope{Kind: ScopeNode}
	case *identityv1.ResourceScope_PrincipalOwned:
		return ResourceScope{Kind: ScopePrincipalOwned, Owner: x.PrincipalOwned.Owner}
	case *identityv1.ResourceScope_Exact:
		r := x.Exact.GetResource()
		if r == nil {
			return ResourceScope{Kind: ScopeExact}
		}
		return ResourceScope{Kind: ScopeExact, Resource: ResourceRef{Node: r.Node, Owner: r.Owner, Kind: r.Kind, CanonicalID: r.CanonicalId}}
	}
	return ResourceScope{}
}
func credentialEnvelope(a *Artifact) (*identityv1.KeyCredential, error) {
	if a == nil || a.kind != KindKeyCredential {
		return nil, ErrInvalidArtifact
	}
	raw, err := a.MarshalBinary()
	if err != nil {
		return nil, err
	}
	var c identityv1.KeyCredential
	if proto.Unmarshal(raw, &c) != nil {
		return nil, ErrInvalidArtifact
	}
	return &c, nil
}
func timestamp(v time.Time) *timestamppb.Timestamp { return timestamppb.New(v.UTC()) }
