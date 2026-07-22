// Package identity verifies and creates the bounded, canonical identity
// artifacts used by Ardents protocol version 1.
package identity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MaxKeyCredentialBytes = identitycontract.MaxKeyCredentialBytes
	MaxArtifactBytes      = identitycontract.MaxArtifactBytes
	PortableClockSkew     = identitycontract.PortableClockSkew
	MaxCredentialLifetime = identitycontract.MaxCredentialLifetime
	MaxGrantLifetime      = identitycontract.MaxGrantLifetime
	MaxDelegationLifetime = identitycontract.MaxDelegationLifetime
	MaxActions            = identitycontract.MaxActions
)

var ErrInvalidArtifact = errors.New("identity artifact is invalid")

var (
	credentialDomain           = []byte(identitycontract.KeyCredentialDomain)
	grantDomain                = []byte(identitycontract.AccessGrantDomain)
	delegationDomain           = []byte(identitycontract.DelegationDomain)
	deviceRevocationDomain     = []byte(identitycontract.DeviceRevocationDomain)
	grantRevocationDomain      = []byte(identitycontract.AccessGrantRevocationDomain)
	delegationRevocationDomain = []byte(identitycontract.DelegationRevocationDomain)
	b32                        = base32.StdEncoding.WithPadding(base32.NoPadding)
)

type Kind string

const (
	KindKeyCredential         Kind = "key-credential"
	KindAccessGrant           Kind = "access-grant"
	KindDelegation            Kind = "delegation"
	KindDeviceRevocation      Kind = "device-revocation"
	KindAccessGrantRevocation Kind = "access-grant-revocation"
	KindDelegationRevocation  Kind = "delegation-revocation"
)

// Artifact is immutable: wire bytes are copied on ingress and egress, and all
// payload accessors return protobuf clones. Its text and JSON forms never expose
// signatures or embedded credentials.
type Artifact struct {
	id      string
	kind    Kind
	raw     []byte
	payload proto.Message
}

func (a *Artifact) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}
func (a *Artifact) Kind() Kind {
	if a == nil {
		return ""
	}
	return a.kind
}
func (a *Artifact) String() string {
	if a == nil {
		return "<nil>"
	}
	return "identity artifact " + a.id + " [redacted]"
}
func (a *Artifact) GoString() string { return a.String() }
func (a *Artifact) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string `json:"id"`
		Kind      Kind   `json:"kind"`
		Protected string `json:"protected"`
	}{a.ID(), a.Kind(), "[redacted]"})
}
func (a *Artifact) MarshalBinary() ([]byte, error) {
	if a == nil {
		return nil, ErrInvalidArtifact
	}
	return bytes.Clone(a.raw), nil
}

func SignKeyCredential(input KeyCredentialSpec, root ed25519.PrivateKey) (*Artifact, error) {
	if len(root) != ed25519.PrivateKeySize {
		return nil, ErrInvalidArtifact
	}
	p := credentialToProto(input)
	sort.Slice(p.Purposes, func(i, j int) bool { return p.Purposes[i] < p.Purposes[j] })
	p.Purposes = compactPurposes(p.Purposes)
	if !bytes.Equal(root.Public().(ed25519.PublicKey), p.RootPublicKey) || validateCredential(p, time.Time{}) != nil {
		return nil, ErrInvalidArtifact
	}
	return sign(p, credentialDomain, "kc1_", KindKeyCredential, root, func(id string, sig []byte) proto.Message {
		return &identityv1.KeyCredential{Id: id, Payload: p, Signature: sig}
	}, MaxKeyCredentialBytes)
}

func ParseKeyCredential(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityv1.KeyCredential)
	if strictUnmarshal(raw, m, MaxKeyCredentialBytes) != nil || validateCredential(m.Payload, now) != nil {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, credentialDomain, "kc1_", KindKeyCredential, ed25519.PublicKey(m.Payload.RootPublicKey))
}

func ParseAccessGrant(raw []byte, issuer ed25519.PublicKey, now time.Time) (*Artifact, error) {
	m := new(identityv1.AccessGrant)
	if strictUnmarshal(raw, m, MaxArtifactBytes) != nil || validateGrant(m.Payload, now) != nil || principalID(issuer) != m.GetPayload().GetIssuer() {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, grantDomain, "ag1_", KindAccessGrant, issuer)
}

func SignDelegation(input DelegationSpec, device ed25519.PrivateKey, now time.Time) (*Artifact, error) {
	if len(device) != ed25519.PrivateKeySize {
		return nil, ErrInvalidArtifact
	}
	credential, err := credentialEnvelope(input.Credential)
	if err != nil {
		return nil, ErrInvalidArtifact
	}
	p := &identityv1.DelegationPayload{Version: identitycontract.Version, Delegator: input.Delegator, Delegatee: input.Delegatee, Audience: audienceToProto(input.Audience), Actions: append([]string(nil), input.Actions...), Scope: scopeToProto(input.Scope), NotBefore: timestamp(input.NotBefore), NotAfter: timestamp(input.NotAfter), Credential: credential}
	sort.Strings(p.Actions)
	p.Actions = compact(p.Actions)
	if validateDelegation(p, now) != nil || !bytes.Equal(device.Public().(ed25519.PublicKey), p.GetCredential().GetPayload().GetDevicePublicKey()) {
		return nil, ErrInvalidArtifact
	}
	return sign(p, delegationDomain, "dl1_", KindDelegation, device, func(id string, sig []byte) proto.Message {
		return &identityv1.Delegation{Id: id, Payload: p, Signature: sig}
	}, MaxArtifactBytes)
}

func ParseDelegation(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityv1.Delegation)
	if strictUnmarshal(raw, m, MaxArtifactBytes) != nil || validateDelegation(m.Payload, now) != nil {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, delegationDomain, "dl1_", KindDelegation, ed25519.PublicKey(m.Payload.Credential.Payload.DevicePublicKey))
}

func ParseDeviceRevocation(raw []byte, issuer ed25519.PublicKey, now time.Time) (*Artifact, error) {
	m := new(identityv1.DeviceRevocation)
	if strictUnmarshal(raw, m, MaxArtifactBytes) != nil || validateDeviceRevocation(m.Payload, issuer, now) != nil {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, deviceRevocationDomain, "dv1_", KindDeviceRevocation, issuer)
}

// knownGrant comes from the repository lookup because v1 does not permit
// preemptive grant revocation. It must be the verified target artifact.
func ParseAccessGrantRevocation(raw []byte, issuer ed25519.PublicKey, now time.Time, knownGrant *Artifact) (*Artifact, error) {
	m := new(identityv1.AccessGrantRevocation)
	if strictUnmarshal(raw, m, MaxArtifactBytes) != nil || validateGrantRevocation(m.Payload, issuer, now) != nil {
		return nil, ErrInvalidArtifact
	}
	if validateGrantRevocationTarget(m.GetPayload(), knownGrant) != nil {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, grantRevocationDomain, "ar1_", KindAccessGrantRevocation, issuer)
}

// Delegation revocations are independently verifiable before the target is known.
func ParseDelegationRevocation(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityv1.DelegationRevocation)
	if strictUnmarshal(raw, m, MaxArtifactBytes) != nil || m.GetPayload().GetCredential().GetPayload() == nil {
		return nil, ErrInvalidArtifact
	}
	public := ed25519.PublicKey(m.Payload.Credential.Payload.DevicePublicKey)
	if validateDelegationRevocation(m.Payload, public, now) != nil {
		return nil, ErrInvalidArtifact
	}
	return verify(raw, m.Id, m.Signature, m.Payload, delegationRevocationDomain, "dr1_", KindDelegationRevocation, public)
}

func sign(payload proto.Message, domain []byte, prefix string, kind Kind, key ed25519.PrivateKey, envelope func(string, []byte) proto.Message, limit int) (*Artifact, error) {
	payloadRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidArtifact
	}
	signed := append(bytes.Clone(domain), payloadRaw...)
	id := artifactID(prefix, signed)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope(id, ed25519.Sign(key, signed)))
	if err != nil || !validWireSize(len(raw), limit) {
		return nil, ErrInvalidArtifact
	}
	return &Artifact{id: id, kind: kind, raw: bytes.Clone(raw), payload: proto.Clone(payload)}, nil
}

func verify(raw []byte, id string, signature []byte, payload proto.Message, domain []byte, prefix string, kind Kind, public ed25519.PublicKey) (*Artifact, error) {
	payloadRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidArtifact
	}
	signed := append(bytes.Clone(domain), payloadRaw...)
	if id != artifactID(prefix, signed) || len(signature) != ed25519.SignatureSize || len(public) != ed25519.PublicKeySize || !ed25519.Verify(public, signed, signature) {
		return nil, ErrInvalidArtifact
	}
	return &Artifact{id: id, kind: kind, raw: bytes.Clone(raw), payload: proto.Clone(payload)}, nil
}

func strictUnmarshal(raw []byte, message proto.Message, limit int) error {
	if !validWireSize(len(raw), limit) {
		return ErrInvalidArtifact
	}
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, message); err != nil || hasUnknown(message) {
		return ErrInvalidArtifact
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ErrInvalidArtifact
	}
	return nil
}

func validWireSize(size, limit int) bool {
	if limit == MaxKeyCredentialBytes {
		return identitycontract.ValidKeyCredentialSize(size)
	}
	if limit == MaxArtifactBytes {
		return identitycontract.ValidArtifactSize(size)
	}
	return false
}

func hasUnknown(m proto.Message) bool {
	if m == nil || len(m.ProtoReflect().GetUnknown()) != 0 {
		return true
	}
	r := m.ProtoReflect()
	fields := r.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.Message() == nil {
			continue
		}
		v := r.Get(fd)
		if fd.IsList() {
			l := v.List()
			for j := 0; j < l.Len(); j++ {
				if hasUnknown(l.Get(j).Message().Interface()) {
					return true
				}
			}
		} else if r.Has(fd) && hasUnknown(v.Message().Interface()) {
			return true
		}
	}
	return false
}

func validateCredential(p *identityv1.KeyCredentialPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || len(p.RootPublicKey) != ed25519.PublicKeySize || len(p.DevicePublicKey) != ed25519.PublicKeySize || bytes.Equal(p.RootPublicKey, p.DevicePublicKey) || len(p.Purposes) != 1 || p.Purposes[0] != identityv1.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE || p.Subject != principalID(p.RootPublicKey) || p.DeviceId != deviceID(p.DevicePublicKey) {
		return ErrInvalidArtifact
	}
	return validateInterval(p.NotBefore, p.NotAfter, MaxCredentialLifetime, now)
}
func validateGrant(p *identityv1.AccessGrantPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.Audience == nil || p.Issuer != p.Audience.Node || validateAudience(p.Audience) != nil || !validPrincipalID(p.Subject) || validateActions(p.Actions, p.Audience.Interface) != nil || validateScope(p.Scope, p.Audience.Node) != nil {
		return ErrInvalidArtifact
	}
	return validateInterval(p.NotBefore, p.NotAfter, MaxGrantLifetime, now)
}
func validateDelegation(p *identityv1.DelegationPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.Audience == nil || p.Audience.Interface != identityv1.Interface_INTERFACE_APPLICATION || validateAudience(p.Audience) != nil || !validPrincipalID(p.Delegator) || !validPrincipalID(p.Delegatee) || p.Delegator == p.Delegatee || validateActions(p.Actions, p.Audience.Interface) != nil || validateScope(p.Scope, p.Audience.Node) != nil || validateInterval(p.NotBefore, p.NotAfter, MaxDelegationLifetime, now) != nil || p.Credential == nil || p.Credential.Payload == nil || p.Credential.Payload.Subject != p.Delegator || validateEmbeddedCredential(p.Credential, now) != nil {
		return ErrInvalidArtifact
	}
	return nil
}
func validateDeviceRevocation(p *identityv1.DeviceRevocationPayload, issuer ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.TargetId != p.TargetDeviceId || !validDeviceID(p.TargetDeviceId) || !validPrincipalID(p.Subject) || validateAudience(p.Audience) != nil || p.Issuer != p.Audience.Node || p.Issuer != principalID(issuer) || validateRevokedAt(p.RevokedAt, now) != nil {
		return ErrInvalidArtifact
	}
	return nil
}
func validateGrantRevocation(p *identityv1.AccessGrantRevocationPayload, issuer ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || !validArtifactID(p.TargetId, identitycontract.AccessGrantPrefix) || validateAudience(p.Audience) != nil || p.Issuer != p.Audience.Node || p.Issuer != principalID(issuer) || validateRevokedAt(p.RevokedAt, now) != nil {
		return ErrInvalidArtifact
	}
	return nil
}
func validateGrantRevocationTarget(p *identityv1.AccessGrantRevocationPayload, target *Artifact) error {
	if p == nil || target == nil || target.kind != KindAccessGrant || target.ID() != p.TargetId {
		return ErrInvalidArtifact
	}
	grant, ok := target.payload.(*identityv1.AccessGrantPayload)
	if !ok || grant.Issuer != p.Issuer || !proto.Equal(grant.Audience, p.Audience) {
		return ErrInvalidArtifact
	}
	return nil
}
func validateDelegationRevocation(p *identityv1.DelegationRevocationPayload, device ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || !validArtifactID(p.TargetId, identitycontract.DelegationPrefix) || validateAudience(p.Audience) != nil || p.Audience.Interface != identityv1.Interface_INTERFACE_APPLICATION || p.Issuer != p.Delegator || !validPrincipalID(p.Delegatee) || p.Delegator == p.Delegatee || p.Credential == nil || p.Credential.Payload == nil || p.Credential.Payload.Subject != p.Delegator || !bytes.Equal(p.Credential.Payload.DevicePublicKey, device) || validateRevokedAt(p.RevokedAt, now) != nil {
		return ErrInvalidArtifact
	}
	// Revocation is permanent. Its signing Credential must have been valid when
	// the revocation was made, not necessarily when the record is imported.
	if validateEmbeddedCredential(p.Credential, p.RevokedAt.AsTime()) != nil {
		return ErrInvalidArtifact
	}
	return nil
}
func validateEmbeddedCredential(c *identityv1.KeyCredential, now time.Time) error {
	if c == nil || hasUnknown(c) || validateCredential(c.Payload, now) != nil {
		return ErrInvalidArtifact
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(c.Payload)
	if err != nil {
		return ErrInvalidArtifact
	}
	signed := append(bytes.Clone(credentialDomain), raw...)
	if c.Id != artifactID("kc1_", signed) || len(c.Signature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(c.Payload.RootPublicKey), signed, c.Signature) {
		return ErrInvalidArtifact
	}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(c)
	if err != nil || len(wire) > MaxKeyCredentialBytes {
		return ErrInvalidArtifact
	}
	return nil
}
func validateAudience(a *identityv1.Audience) error {
	if a == nil || !validPrincipalID(a.Node) || a.ProtocolMajor != identitycontract.ProtocolMajor || (a.Interface != identityv1.Interface_INTERFACE_OPERATOR && a.Interface != identityv1.Interface_INTERFACE_APPLICATION) {
		return ErrInvalidArtifact
	}
	return nil
}
func validateScope(s *identityv1.ResourceScope, node string) error {
	if s == nil {
		return ErrInvalidArtifact
	}
	switch x := s.Scope.(type) {
	case *identityv1.ResourceScope_Node:
		if x.Node == nil {
			return ErrInvalidArtifact
		}
	case *identityv1.ResourceScope_PrincipalOwned:
		if x.PrincipalOwned == nil || !validPrincipalID(x.PrincipalOwned.Owner) {
			return ErrInvalidArtifact
		}
	case *identityv1.ResourceScope_Exact:
		r := x.Exact.GetResource()
		if r == nil {
			return ErrInvalidArtifact
		}
		contract, known := identitycontract.LookupResourceKind(r.GetKind())
		ownerPresent := r.Owner != ""
		if r.Node != node || !known || len(r.CanonicalId) > identitycontract.MaxCanonicalResourceIDBytes || (!contract.AllowEmptyID && len(r.CanonicalId) == 0) || ownerPresent != contract.OwnerRequired || (ownerPresent && !validPrincipalID(r.Owner)) {
			return ErrInvalidArtifact
		}
	default:
		return ErrInvalidArtifact
	}
	return nil
}
func validateActions(actions []string, i identityv1.Interface) error {
	if !identitycontract.ValidActionCount(len(actions)) || !sort.StringsAreSorted(actions) {
		return ErrInvalidArtifact
	}
	for n, a := range actions {
		if n > 0 && actions[n-1] == a || !knownAction(i, a) {
			return ErrInvalidArtifact
		}
	}
	return nil
}
func validateInterval(a, b *timestamppb.Timestamp, max time.Duration, now time.Time) error {
	if !canonicalTimestamp(a) || !canonicalTimestamp(b) {
		return ErrInvalidArtifact
	}
	start, end := a.AsTime(), b.AsTime()
	if !end.After(start) || end.Sub(start) > max || (!now.IsZero() && (now.Before(start.Add(-PortableClockSkew)) || !now.Before(end.Add(PortableClockSkew)))) {
		return ErrInvalidArtifact
	}
	return nil
}
func validateRevokedAt(v *timestamppb.Timestamp, now time.Time) error {
	if !canonicalTimestamp(v) || (!now.IsZero() && v.AsTime().After(now.Add(PortableClockSkew))) {
		return ErrInvalidArtifact
	}
	return nil
}
func canonicalTimestamp(v *timestamppb.Timestamp) bool {
	if v == nil || v.Nanos != 0 || !v.IsValid() {
		return false
	}
	t := v.AsTime()
	return !t.Before(time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()) && t.Before(time.Unix(identitycontract.UpperTimestampUnix, 0).UTC())
}

func principalID(public []byte) string {
	if len(public) != ed25519.PublicKeySize {
		return ""
	}
	return digestID("p1_", []byte("ardents:principal:v1\x00"), public)
}

func deviceID(public []byte) string {
	if len(public) != ed25519.PublicKeySize {
		return ""
	}
	return digestID("d1_", []byte("ardents:device:v1\x00"), public)
}

func digestID(prefix string, domain, public []byte) string {
	material := append(bytes.Clone(domain), byte(1))
	material = append(material, public...)
	sum := sha256.Sum256(material)
	return prefix + strings.ToLower(b32.EncodeToString(sum[:]))
}
func artifactID(prefix string, signed []byte) string {
	sum := sha256.Sum256(signed)
	return prefix + strings.ToLower(b32.EncodeToString(sum[:]))
}
func validPrincipalID(id string) bool        { return validDigestID(id, "p1_") }
func validDeviceID(id string) bool           { return validDigestID(id, "d1_") }
func validArtifactID(id, prefix string) bool { return validDigestID(id, prefix) }
func validDigestID(id, prefix string) bool {
	if len(id) != len(prefix)+52 || !strings.HasPrefix(id, prefix) || id != strings.ToLower(id) {
		return false
	}
	raw, err := b32.DecodeString(strings.ToUpper(id[len(prefix):]))
	return err == nil && len(raw) == sha256.Size && !bytes.Equal(raw, make([]byte, sha256.Size)) && strings.ToLower(b32.EncodeToString(raw)) == id[len(prefix):]
}
func compact(v []string) []string {
	out := v[:0]
	for _, s := range v {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func compactPurposes(v []identityv1.CredentialPurpose) []identityv1.CredentialPurpose {
	out := v[:0]
	for _, purpose := range v {
		if len(out) == 0 || out[len(out)-1] != purpose {
			out = append(out, purpose)
		}
	}
	return out
}

func knownAction(i identityv1.Interface, a string) bool {
	if i == identityv1.Interface_INTERFACE_APPLICATION {
		return identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, a)
	}
	if i == identityv1.Interface_INTERFACE_OPERATOR {
		return identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, a)
	}
	return false
}
