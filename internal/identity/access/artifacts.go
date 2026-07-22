// Package access owns canonical, signed identity artifacts. Generated protobuf
// messages are wire DTOs; callers receive redacting immutable wrappers.
package access

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"sort"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	maxCredentialBytes = identitycontract.MaxKeyCredentialBytes
	maxArtifactBytes   = identitycontract.MaxArtifactBytes
	portableSkew       = identitycontract.PortableClockSkew
	maxCredentialLife  = identitycontract.MaxCredentialLifetime
	maxGrantLife       = identitycontract.MaxGrantLifetime
	maxDelegationLife  = identitycontract.MaxDelegationLifetime
)

var (
	credentialDomain           = []byte(identitycontract.KeyCredentialDomain)
	grantDomain                = []byte(identitycontract.AccessGrantDomain)
	delegationDomain           = []byte(identitycontract.DelegationDomain)
	deviceRevocationDomain     = []byte(identitycontract.DeviceRevocationDomain)
	grantRevocationDomain      = []byte(identitycontract.AccessGrantRevocationDomain)
	delegationRevocationDomain = []byte(identitycontract.DelegationRevocationDomain)
	b32                        = base32.StdEncoding.WithPadding(base32.NoPadding)
	errInvalid                 = errors.New("identity artifact is invalid")
)

type Artifact struct {
	id      string
	raw     []byte
	payload proto.Message
}

func (a *Artifact) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}
func (a *Artifact) String() string {
	if a == nil {
		return "<nil>"
	}
	return "identity artifact " + a.id + " [redacted]"
}
func (a *Artifact) GoString() string { return a.String() }
func (a *Artifact) MarshalJSON() ([]byte, error) {
	return []byte(`{"id":"` + a.ID() + `","protected":"[redacted]"}`), nil
}
func (a *Artifact) MarshalBinary() ([]byte, error) {
	if a == nil {
		return nil, errInvalid
	}
	return append([]byte(nil), a.raw...), nil
}
func (a *Artifact) AccessGrantPayload() *identityprotocol.AccessGrantPayload {
	if a == nil {
		return nil
	}
	p, _ := a.payload.(*identityprotocol.AccessGrantPayload)
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*identityprotocol.AccessGrantPayload)
}
func (a *Artifact) KeyCredentialPayload() *identityprotocol.KeyCredentialPayload {
	if a == nil {
		return nil
	}
	p, _ := a.payload.(*identityprotocol.KeyCredentialPayload)
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*identityprotocol.KeyCredentialPayload)
}
func (a *Artifact) DeviceRevocationPayload() *identityprotocol.DeviceRevocationPayload {
	if a == nil {
		return nil
	}
	p, _ := a.payload.(*identityprotocol.DeviceRevocationPayload)
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*identityprotocol.DeviceRevocationPayload)
}
func (a *Artifact) AccessGrantRevocationPayload() *identityprotocol.AccessGrantRevocationPayload {
	if a == nil {
		return nil
	}
	p, _ := a.payload.(*identityprotocol.AccessGrantRevocationPayload)
	if p == nil {
		return nil
	}
	return proto.Clone(p).(*identityprotocol.AccessGrantRevocationPayload)
}

func SignKeyCredential(input *identityprotocol.KeyCredentialPayload, key ed25519.PrivateKey) (*Artifact, error) {
	p, ok := cloneCredential(input)
	if !ok || hasUnknown(input) || len(key) != ed25519.PrivateKeySize || !bytes.Equal(key.Public().(ed25519.PublicKey), p.GetRootPublicKey()) {
		return nil, errInvalid
	}
	sort.Slice(p.Purposes, func(i, j int) bool { return p.Purposes[i] < p.Purposes[j] })
	p.Purposes = compactPurposes(p.Purposes)
	if err := validateCredential(p, time.Time{}); err != nil {
		return nil, err
	}
	return signEnvelope(p, credentialDomain, "kc1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.KeyCredential{Id: id, Payload: p, Signature: sig}
	}, maxCredentialBytes)
}

func ParseAndVerifyKeyCredential(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityprotocol.KeyCredential)
	if err := strictUnmarshal(raw, m, maxCredentialBytes); err != nil {
		return nil, err
	}
	p := m.GetPayload()
	if err := validateCredential(p, now); err != nil {
		return nil, err
	}
	return verifyEnvelope(raw, m.GetId(), m.GetSignature(), p, credentialDomain, "kc1_", ed25519.PublicKey(p.GetRootPublicKey()))
}

func SignAccessGrant(input *identityprotocol.AccessGrantPayload, key ed25519.PrivateKey) (*Artifact, error) {
	p, ok := cloneGrant(input)
	if !ok || hasUnknown(input) || len(key) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	sort.Strings(p.Actions)
	p.Actions = compact(p.Actions)
	if err := validateGrant(p, time.Time{}); err != nil {
		return nil, err
	}
	derived, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil || derived.String() != p.Issuer {
		return nil, errInvalid
	}
	return signEnvelope(p, grantDomain, "ag1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.AccessGrant{Id: id, Payload: p, Signature: sig}
	}, maxArtifactBytes)
}

func ParseAndVerifyAccessGrant(raw []byte, issuer ed25519.PublicKey, now time.Time) (*Artifact, error) {
	m := new(identityprotocol.AccessGrant)
	if err := strictUnmarshal(raw, m, maxArtifactBytes); err != nil {
		return nil, err
	}
	p := m.GetPayload()
	if err := validateGrant(p, now); err != nil {
		return nil, err
	}
	derived, err := identityprincipal.FromEd25519PublicKey(issuer)
	if err != nil || derived.String() != p.GetIssuer() {
		return nil, errInvalid
	}
	return verifyEnvelope(raw, m.GetId(), m.GetSignature(), p, grantDomain, "ag1_", issuer)
}

func SignDelegation(input *identityprotocol.DelegationPayload, key ed25519.PrivateKey, now time.Time) (*Artifact, error) {
	if input == nil || hasUnknown(input) || len(key) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	p := proto.Clone(input).(*identityprotocol.DelegationPayload)
	sort.Strings(p.Actions)
	p.Actions = compact(p.Actions)
	if err := validateDelegation(p, now); err != nil {
		return nil, err
	}
	if !bytes.Equal(key.Public().(ed25519.PublicKey), p.Credential.Payload.DevicePublicKey) {
		return nil, errInvalid
	}
	return signEnvelope(p, delegationDomain, "dl1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.Delegation{Id: id, Payload: p, Signature: sig}
	}, maxArtifactBytes)
}

func ParseAndVerifyDelegation(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityprotocol.Delegation)
	if err := strictUnmarshal(raw, m, maxArtifactBytes); err != nil {
		return nil, err
	}
	p := m.GetPayload()
	if err := validateDelegation(p, now); err != nil {
		return nil, err
	}
	return verifyEnvelope(raw, m.GetId(), m.GetSignature(), p, delegationDomain, "dl1_", ed25519.PublicKey(p.Credential.Payload.DevicePublicKey))
}

func SignDeviceRevocation(input *identityprotocol.DeviceRevocationPayload, key ed25519.PrivateKey, now time.Time) (*Artifact, error) {
	if input == nil || hasUnknown(input) || len(key) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	p := proto.Clone(input).(*identityprotocol.DeviceRevocationPayload)
	if err := validateDeviceRevocation(p, key.Public().(ed25519.PublicKey), now); err != nil {
		return nil, err
	}
	return signEnvelope(p, deviceRevocationDomain, "dv1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.DeviceRevocation{Id: id, Payload: p, Signature: sig}
	}, maxArtifactBytes)
}

func ParseAndVerifyDeviceRevocation(raw []byte, issuer ed25519.PublicKey, now time.Time) (*Artifact, error) {
	m := new(identityprotocol.DeviceRevocation)
	if err := strictUnmarshal(raw, m, maxArtifactBytes); err != nil {
		return nil, err
	}
	if err := validateDeviceRevocation(m.Payload, issuer, now); err != nil {
		return nil, err
	}
	return verifyEnvelope(raw, m.Id, m.Signature, m.Payload, deviceRevocationDomain, "dv1_", issuer)
}

func SignAccessGrantRevocation(input *identityprotocol.AccessGrantRevocationPayload, key ed25519.PrivateKey, now time.Time, knownGrant *Artifact) (*Artifact, error) {
	if input == nil || hasUnknown(input) || len(key) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	p := proto.Clone(input).(*identityprotocol.AccessGrantRevocationPayload)
	if err := validateGrantRevocation(p, key.Public().(ed25519.PublicKey), now); err != nil {
		return nil, err
	}
	if err := validateGrantRevocationTarget(p, knownGrant); err != nil {
		return nil, err
	}
	return signEnvelope(p, grantRevocationDomain, "ar1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.AccessGrantRevocation{Id: id, Payload: p, Signature: sig}
	}, maxArtifactBytes)
}

// knownGrant is supplied by the repository lookup because v1 forbids
// preemptive Access Grant revocation. It must be the verified target artifact.
func ParseAndVerifyAccessGrantRevocation(raw []byte, issuer ed25519.PublicKey, now time.Time, knownGrant *Artifact) (*Artifact, error) {
	m := new(identityprotocol.AccessGrantRevocation)
	if err := strictUnmarshal(raw, m, maxArtifactBytes); err != nil {
		return nil, err
	}
	if err := validateGrantRevocationTarget(m.GetPayload(), knownGrant); err != nil {
		return nil, errInvalid
	}
	if err := validateGrantRevocation(m.Payload, issuer, now); err != nil {
		return nil, err
	}
	return verifyEnvelope(raw, m.Id, m.Signature, m.Payload, grantRevocationDomain, "ar1_", issuer)
}

func SignDelegationRevocation(input *identityprotocol.DelegationRevocationPayload, key ed25519.PrivateKey, now time.Time) (*Artifact, error) {
	if input == nil || hasUnknown(input) || len(key) != ed25519.PrivateKeySize {
		return nil, errInvalid
	}
	p := proto.Clone(input).(*identityprotocol.DelegationRevocationPayload)
	if err := validateDelegationRevocation(p, key.Public().(ed25519.PublicKey), now); err != nil {
		return nil, err
	}
	return signEnvelope(p, delegationRevocationDomain, "dr1_", key, func(id string, sig []byte) proto.Message {
		return &identityprotocol.DelegationRevocation{Id: id, Payload: p, Signature: sig}
	}, maxArtifactBytes)
}

// Delegation revocations may be verified before the target Delegation is known.
func ParseAndVerifyDelegationRevocation(raw []byte, now time.Time) (*Artifact, error) {
	m := new(identityprotocol.DelegationRevocation)
	if err := strictUnmarshal(raw, m, maxArtifactBytes); err != nil {
		return nil, err
	}
	p := m.Payload
	if p == nil || p.Credential == nil || p.Credential.Payload == nil {
		return nil, errInvalid
	}
	public := ed25519.PublicKey(p.Credential.Payload.DevicePublicKey)
	if err := validateDelegationRevocation(p, public, now); err != nil {
		return nil, err
	}
	return verifyEnvelope(raw, m.Id, m.Signature, p, delegationRevocationDomain, "dr1_", public)
}

func signEnvelope(payload proto.Message, domain []byte, prefix string, key ed25519.PrivateKey, envelope func(string, []byte) proto.Message, limit int) (*Artifact, error) {
	payloadRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		return nil, errInvalid
	}
	signed := append(append([]byte(nil), domain...), payloadRaw...)
	id := artifactID(prefix, signed)
	wire := envelope(id, ed25519.Sign(key, signed))
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(wire)
	if err != nil || !validWireSize(len(raw), limit) {
		return nil, errInvalid
	}
	return &Artifact{id: id, raw: raw, payload: proto.Clone(payload)}, nil
}

func verifyEnvelope(raw []byte, id string, signature []byte, payload proto.Message, domain []byte, prefix string, public ed25519.PublicKey) (*Artifact, error) {
	payloadRaw, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		return nil, errInvalid
	}
	signed := append(append([]byte(nil), domain...), payloadRaw...)
	if id != artifactID(prefix, signed) || len(signature) != ed25519.SignatureSize || len(public) != ed25519.PublicKeySize || !ed25519.Verify(public, signed, signature) {
		return nil, errInvalid
	}
	return &Artifact{id: id, raw: append([]byte(nil), raw...), payload: proto.Clone(payload)}, nil
}

func strictUnmarshal(raw []byte, message proto.Message, limit int) error {
	if !validWireSize(len(raw), limit) {
		return errInvalid
	}
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(raw, message); err != nil || hasUnknown(message) {
		return errInvalid
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil || !bytes.Equal(canonical, raw) {
		return errInvalid
	}
	return nil
}

func validWireSize(size, limit int) bool {
	if limit == maxCredentialBytes {
		return identitycontract.ValidKeyCredentialSize(size)
	}
	if limit == maxArtifactBytes {
		return identitycontract.ValidArtifactSize(size)
	}
	return false
}

func hasUnknown(m proto.Message) bool {
	if m == nil || len(m.ProtoReflect().GetUnknown()) != 0 {
		return true
	}
	found := false
	// Unknown fields in every nested message are caught by walking known fields.
	fields := m.ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len() && !found; i++ {
		fd := fields.Get(i)
		if fd.Message() == nil {
			continue
		}
		v := m.ProtoReflect().Get(fd)
		if fd.IsList() {
			l := v.List()
			for j := 0; j < l.Len(); j++ {
				if hasUnknown(l.Get(j).Message().Interface()) {
					found = true
					break
				}
			}
		} else if m.ProtoReflect().Has(fd) && hasUnknown(v.Message().Interface()) {
			found = true
		}
	}
	return found
}

func validateCredential(p *identityprotocol.KeyCredentialPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || len(p.RootPublicKey) != ed25519.PublicKeySize || len(p.DevicePublicKey) != ed25519.PublicKeySize || bytes.Equal(p.RootPublicKey, p.DevicePublicKey) || len(p.Purposes) != 1 || p.Purposes[0] != identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE {
		return errInvalid
	}
	subject, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(p.RootPublicKey))
	if err != nil || subject.String() != p.Subject {
		return errInvalid
	}
	device, err := identityprincipal.DeviceFromEd25519PublicKey(ed25519.PublicKey(p.DevicePublicKey))
	if err != nil || device.String() != p.DeviceId {
		return errInvalid
	}
	return validateInterval(p.NotBefore, p.NotAfter, maxCredentialLife, now)
}

func validateGrant(p *identityprotocol.AccessGrantPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.Audience == nil || p.Issuer != p.Audience.Node || validateAudience(p.Audience) != nil {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(p.Subject); err != nil {
		return errInvalid
	}
	if !identitycontract.ValidActionCount(len(p.Actions)) || !sort.StringsAreSorted(p.Actions) {
		return errInvalid
	}
	for i, a := range p.Actions {
		if i > 0 && p.Actions[i-1] == a || !knownAction(p.Audience.Interface, a) {
			return errInvalid
		}
	}
	if err := validateScope(p.Scope, p.Audience.Node); err != nil {
		return err
	}
	return validateInterval(p.NotBefore, p.NotAfter, maxGrantLife, now)
}

func validateDelegation(p *identityprotocol.DelegationPayload, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.Audience == nil || p.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION || validateAudience(p.Audience) != nil {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(p.Delegator); err != nil {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(p.Delegatee); err != nil || p.Delegatee == p.Delegator {
		return errInvalid
	}
	if !identitycontract.ValidActionCount(len(p.Actions)) || !sort.StringsAreSorted(p.Actions) {
		return errInvalid
	}
	for i, action := range p.Actions {
		if i > 0 && p.Actions[i-1] == action || !knownAction(p.Audience.Interface, action) {
			return errInvalid
		}
	}
	if err := validateScope(p.Scope, p.Audience.Node); err != nil {
		return err
	}
	if err := validateInterval(p.NotBefore, p.NotAfter, maxDelegationLife, now); err != nil {
		return err
	}
	credential := p.GetCredential()
	if credential == nil || hasUnknown(credential) || credential.GetPayload() == nil || credential.Payload.Subject != p.Delegator {
		return errInvalid
	}
	return validateEmbeddedCredential(credential, now)
}

func validateDeviceRevocation(p *identityprotocol.DeviceRevocationPayload, issuer ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || p.TargetId != p.TargetDeviceId || validateAudience(p.Audience) != nil || p.Issuer != p.Audience.Node {
		return errInvalid
	}
	if _, err := identityprincipal.ParseDeviceID(p.TargetDeviceId); err != nil {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(p.Subject); err != nil {
		return errInvalid
	}
	if err := validateIssuer(p.Issuer, issuer); err != nil {
		return err
	}
	return validateRevokedAt(p.RevokedAt, now)
}
func validateGrantRevocation(p *identityprotocol.AccessGrantRevocationPayload, issuer ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || !validArtifactID(p.TargetId, identitycontract.AccessGrantPrefix) || validateAudience(p.Audience) != nil || p.Issuer != p.Audience.Node {
		return errInvalid
	}
	if err := validateIssuer(p.Issuer, issuer); err != nil {
		return err
	}
	return validateRevokedAt(p.RevokedAt, now)
}
func validateGrantRevocationTarget(p *identityprotocol.AccessGrantRevocationPayload, target *Artifact) error {
	if p == nil || target == nil || target.ID() != p.TargetId {
		return errInvalid
	}
	grant := target.AccessGrantPayload()
	if grant == nil || grant.Issuer != p.Issuer || !proto.Equal(grant.Audience, p.Audience) {
		return errInvalid
	}
	return nil
}
func validateDelegationRevocation(p *identityprotocol.DelegationRevocationPayload, device ed25519.PublicKey, now time.Time) error {
	if p == nil || p.Version != identitycontract.Version || !validArtifactID(p.TargetId, identitycontract.DelegationPrefix) || validateAudience(p.Audience) != nil || p.Audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION || p.Issuer != p.Delegator {
		return errInvalid
	}
	if _, err := identityprincipal.Parse(p.Delegatee); err != nil {
		return errInvalid
	}
	if p.Credential == nil || p.Credential.Payload == nil || p.Credential.Payload.Subject != p.Delegator || !bytes.Equal(p.Credential.Payload.DevicePublicKey, device) {
		return errInvalid
	}
	if p.RevokedAt == nil {
		return errInvalid
	}
	if err := validateEmbeddedCredential(p.Credential, p.RevokedAt.AsTime()); err != nil {
		return err
	}
	return validateRevokedAt(p.RevokedAt, now)
}
func validateEmbeddedCredential(c *identityprotocol.KeyCredential, now time.Time) error {
	if c == nil || hasUnknown(c) || c.Payload == nil {
		return errInvalid
	}
	if err := validateCredential(c.Payload, now); err != nil {
		return err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(c.Payload)
	if err != nil {
		return errInvalid
	}
	signed := append(append([]byte(nil), credentialDomain...), raw...)
	if c.Id != artifactID("kc1_", signed) || len(c.Signature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(c.Payload.RootPublicKey), signed, c.Signature) {
		return errInvalid
	}
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(c)
	if err != nil || len(wire) > maxCredentialBytes {
		return errInvalid
	}
	return nil
}
func validateIssuer(id string, key ed25519.PublicKey) error {
	derived, err := identityprincipal.FromEd25519PublicKey(key)
	if err != nil || derived.String() != id {
		return errInvalid
	}
	return nil
}
func validateRevokedAt(ts *timestamppb.Timestamp, now time.Time) error {
	if ts == nil || ts.Nanos != 0 || !ts.IsValid() {
		return errInvalid
	}
	v := ts.AsTime()
	min := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	if v.Before(min) || !v.Before(upper) || (!now.IsZero() && v.After(now.Add(portableSkew))) {
		return errInvalid
	}
	return nil
}
func validArtifactID(id, prefix string) bool {
	if len(id) != len(prefix)+52 || !strings.HasPrefix(id, prefix) || id != strings.ToLower(id) {
		return false
	}
	raw, err := b32.DecodeString(strings.ToUpper(id[len(prefix):]))
	return err == nil && len(raw) == sha256.Size && !bytes.Equal(raw, make([]byte, sha256.Size)) && strings.ToLower(b32.EncodeToString(raw)) == id[len(prefix):]
}

func validateAudience(a *identityprotocol.Audience) error {
	if _, err := identityprincipal.Parse(a.GetNode()); err != nil || a.ProtocolMajor != identitycontract.ProtocolMajor || (a.Interface != identityprotocol.Interface_INTERFACE_OPERATOR && a.Interface != identityprotocol.Interface_INTERFACE_APPLICATION) {
		return errInvalid
	}
	return nil
}

func validateScope(s *identityprotocol.ResourceScope, node string) error {
	if s == nil {
		return errInvalid
	}
	switch x := s.Scope.(type) {
	case *identityprotocol.ResourceScope_Node:
		if x.Node == nil {
			return errInvalid
		}
	case *identityprotocol.ResourceScope_PrincipalOwned:
		if x.PrincipalOwned == nil {
			return errInvalid
		}
		if _, err := identityprincipal.Parse(x.PrincipalOwned.Owner); err != nil {
			return errInvalid
		}
	case *identityprotocol.ResourceScope_Exact:
		r := x.Exact.GetResource()
		if r == nil {
			return errInvalid
		}
		contract, known := identitycontract.LookupResourceKind(r.Kind)
		ownerPresent := r.Owner != ""
		if r.Node != node || !known || len(r.CanonicalId) > identitycontract.MaxCanonicalResourceIDBytes || (!contract.AllowEmptyID && len(r.CanonicalId) == 0) || ownerPresent != contract.OwnerRequired {
			return errInvalid
		}
		if r.Owner != "" {
			if _, err := identityprincipal.Parse(r.Owner); err != nil {
				return errInvalid
			}
		}
	default:
		return errInvalid
	}
	return nil
}

func validateInterval(a, b *timestamppb.Timestamp, max time.Duration, now time.Time) error {
	if a == nil || b == nil || a.Nanos != 0 || b.Nanos != 0 || !a.IsValid() || !b.IsValid() {
		return errInvalid
	}
	start, end := a.AsTime(), b.AsTime()
	min := time.Unix(identitycontract.LowerTimestampUnix, 0).UTC()
	upper := time.Unix(identitycontract.UpperTimestampUnix, 0).UTC()
	if start.Before(min) || !start.Before(upper) || end.Before(min) || !end.Before(upper) || !end.After(start) || end.Sub(start) > max {
		return errInvalid
	}
	if !now.IsZero() && (now.Before(start.Add(-portableSkew)) || !now.Before(end.Add(portableSkew))) {
		return errInvalid
	}
	return nil
}

func cloneCredential(p *identityprotocol.KeyCredentialPayload) (*identityprotocol.KeyCredentialPayload, bool) {
	if p == nil {
		return nil, false
	}
	return proto.Clone(p).(*identityprotocol.KeyCredentialPayload), true
}
func cloneGrant(p *identityprotocol.AccessGrantPayload) (*identityprotocol.AccessGrantPayload, bool) {
	if p == nil {
		return nil, false
	}
	return proto.Clone(p).(*identityprotocol.AccessGrantPayload), true
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
func compactPurposes(v []identityprotocol.CredentialPurpose) []identityprotocol.CredentialPurpose {
	out := v[:0]
	for _, purpose := range v {
		if len(out) == 0 || out[len(out)-1] != purpose {
			out = append(out, purpose)
		}
	}
	return out
}
func artifactID(prefix string, signed []byte) string {
	sum := sha256.Sum256(signed)
	return prefix + strings.ToLower(b32.EncodeToString(sum[:]))
}

func knownAction(i identityprotocol.Interface, a string) bool {
	if i == identityprotocol.Interface_INTERFACE_APPLICATION {
		return identitycontract.IsRegisteredAction(identitycontract.InterfaceApplication, a)
	}
	if i == identityprotocol.Interface_INTERFACE_OPERATOR {
		return identitycontract.IsRegisteredAction(identitycontract.InterfaceOperator, a)
	}
	return false
}
