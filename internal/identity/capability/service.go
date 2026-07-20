package capability

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	identityapi "ardents/internal/identity/api"
)

type Service struct {
	mu             sync.Mutex
	store          *Store
	ledger         ledger
	refKey         [32]byte
	clock          func() time.Time
	issuers        map[string]ed25519.PublicKey
	localPrincipal string
	admission      identityapi.CapabilityAdmission
}

type Status struct {
	Scope      identityapi.CapabilityScope
	Generation uint32
	State      string
	NotAfter   time.Time
}

func NewService(path string, storeKey []byte, localPrincipal string, issuers map[string]ed25519.PublicKey, admission identityapi.CapabilityAdmission, clock func() time.Time) (*Service, error) {
	if !validPrincipal(localPrincipal) {
		return nil, fmt.Errorf("local capability principal is invalid")
	}
	if admission == nil {
		return nil, fmt.Errorf("capability policy admission is required")
	}
	store, err := newStore(path, storeKey)
	if err != nil {
		return nil, err
	}
	stored, err := store.load()
	if err != nil {
		return nil, err
	}
	if clock == nil {
		clock = time.Now
	}
	service := &Service{
		store: store, ledger: stored, clock: clock,
		issuers: cloneIssuers(issuers), localPrincipal: localPrincipal, admission: admission,
	}
	refKey, err := hkdf.Key(sha256.New, storeKey, nil, "ardents-capability-local-reference/1", len(service.refKey))
	if err != nil {
		return nil, err
	}
	copy(service.refKey[:], refKey)
	return service, nil
}

func (s *Service) ImportGrant(grant identityapi.CapabilityGrant) (identityapi.CapabilityRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	issuerPublic, ok := s.issuers[grant.IssuerPrincipal]
	if !ok {
		return "", capabilityError(CodeIssuerUntrusted, "capability issuer is not trusted")
	}
	if err := validateGrant(grant, issuerPublic); err != nil {
		return "", capabilityError(CodeInvalid, err.Error())
	}
	if grant.SubjectPrincipal != s.localPrincipal {
		return "", capabilityError(CodeScopeDenied, "capability grant is bound to another subject")
	}
	ref := s.reference(grant)
	if err := s.rejectConflictingGrant(ref, grant); err != nil {
		return "", err
	}
	if rev, ok := s.ledger.Revocations[grantIDKey(grant.GrantID)]; ok &&
		rev.IssuerPrincipal != grant.IssuerPrincipal {
		return "", capabilityError(CodeInvalid, "revocation issuer does not match grant issuer")
	}
	next := cloneLedger(s.ledger)
	next.Grants[string(ref)] = persistGrant(grant)
	next.SenderGrants[grantIDKey(grant.GrantID)] = persistGrant(grant)
	if err := s.store.save(next); err != nil {
		return "", err
	}
	s.ledger = next
	return ref, nil
}

func (s *Service) ApplyRevocation(rev identityapi.CapabilityRevocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	issuerPublic, ok := s.issuers[rev.IssuerPrincipal]
	if !ok {
		return capabilityError(CodeIssuerUntrusted, "capability issuer is not trusted")
	}
	if err := validateRevocation(rev, issuerPublic); err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
	if err := s.checkRevocationIssuer(rev); err != nil {
		return err
	}
	next := cloneLedger(s.ledger)
	key := grantIDKey(rev.GrantID)
	if current, ok := next.Revocations[key]; ok && current.RevokedAt.Before(rev.RevokedAt) {
		return capabilityError(CodeInvalid, "revocation cannot move forward")
	}
	next.Revocations[key] = persistRevocation(rev)
	if err := s.store.save(next); err != nil {
		return err
	}
	s.ledger = next
	return nil
}

func (s *Service) ResolveCapability(use identityapi.CapabilityUse) (identityapi.ResolvedCapability, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.ledger.Grants[string(use.Ref)]
	if !ok {
		return identityapi.ResolvedCapability{}, capabilityError(CodeMissing, "capability reference not found")
	}
	grant, err := stored.restore()
	if err != nil {
		return identityapi.ResolvedCapability{}, capabilityError(CodeInvalid, err.Error())
	}
	at := use.At
	if at.IsZero() {
		at = s.clock().UTC()
	}
	if err := s.authorize(grant, use, at); err != nil {
		return identityapi.ResolvedCapability{}, err
	}
	if err := s.admission.AllowCapabilityUse(use); err != nil {
		return identityapi.ResolvedCapability{}, capabilityError(CodeScopeDenied, "capability use denied by policy")
	}
	return resolved(use.Ref, grant), nil
}

func (s *Service) Statuses(at time.Time) []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at.IsZero() {
		at = s.clock().UTC()
	}
	refs := make([]string, 0, len(s.ledger.Grants))
	for ref := range s.ledger.Grants {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	out := make([]Status, 0, len(refs))
	for _, ref := range refs {
		stored := s.ledger.Grants[ref]
		grant, err := stored.restore()
		if err != nil {
			continue
		}
		out = append(out, Status{
			Scope: grant.Scope, Generation: grant.Generation,
			State: s.state(grant, at), NotAfter: grant.NotAfter,
		})
	}
	return out
}

func (s *Service) reference(grant identityapi.CapabilityGrant) identityapi.CapabilityRef {
	mac := hmac.New(sha256.New, s.refKey[:])
	mac.Write([]byte("ardents-capability-ref/1"))
	mac.Write(grant.ChannelID[:])
	mac.Write(grant.GrantID[:])
	mac.Write([]byte(grant.SubjectPrincipal))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(mac.Sum(nil)[:20])
	encoded = strings.ToLower(encoded)
	return identityapi.CapabilityRef("cap_" + encoded)
}

func grantIDKey(id [16]byte) string { return hex.EncodeToString(id[:]) }

func resolved(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant) identityapi.ResolvedCapability {
	return identityapi.ResolvedCapability{
		Ref: ref, ChannelID: grant.ChannelID, Generation: grant.Generation,
		GrantID: grant.GrantID, Subject: grant.SubjectPrincipal,
		Permissions: grant.Permissions, Scope: grant.Scope, Secret: grant.Secret,
	}
}

func cloneLedger(source ledger) ledger {
	out := emptyLedger()
	out.DeliveryPrivateKey = append([]byte(nil), source.DeliveryPrivateKey...)
	for key, value := range source.Grants {
		out.Grants[key] = value
	}
	for key, value := range source.SenderGrants {
		out.SenderGrants[key] = value
	}
	for key, value := range source.Revocations {
		out.Revocations[key] = value
	}
	return out
}

func cloneIssuers(source map[string]ed25519.PublicKey) map[string]ed25519.PublicKey {
	out := make(map[string]ed25519.PublicKey, len(source))
	for principal, public := range source {
		out[principal] = append(ed25519.PublicKey(nil), public...)
	}
	return out
}

func (s *Service) rejectConflictingGrant(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant) error {
	current, ok := s.ledger.Grants[string(ref)]
	if !ok {
		return nil
	}
	existing, err := current.restore()
	if err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
	want, _ := canonicalGrant(existing)
	got, _ := canonicalGrant(grant)
	if !hmac.Equal(want, got) || !hmac.Equal(existing.Signature, grant.Signature) {
		return capabilityError(CodeInvalid, "grant identifier conflicts with retained grant")
	}
	return nil
}

func (s *Service) checkRevocationIssuer(rev identityapi.CapabilityRevocation) error {
	for _, stored := range s.ledger.Grants {
		if stored.GrantID == rev.GrantID && stored.IssuerPrincipal != rev.IssuerPrincipal {
			return capabilityError(CodeInvalid, "revocation issuer does not match grant issuer")
		}
	}
	for _, stored := range s.ledger.SenderGrants {
		if stored.GrantID == rev.GrantID && stored.IssuerPrincipal != rev.IssuerPrincipal {
			return capabilityError(CodeInvalid, "revocation issuer does not match sender grant issuer")
		}
	}
	return nil
}

func (s *Service) authorize(grant identityapi.CapabilityGrant, use identityapi.CapabilityUse, at time.Time) error {
	if use.Subject != grant.SubjectPrincipal || use.Scope != grant.Scope ||
		use.Permission == 0 || use.Permission&^grant.Permissions != 0 {
		return capabilityError(CodeScopeDenied, "capability subject, scope, or permission denied")
	}
	if at.Before(grant.NotBefore) {
		return capabilityError(CodeNotYetValid, "capability is not yet valid")
	}
	if !at.Before(grant.NotAfter) {
		return capabilityError(CodeExpired, "capability is expired")
	}
	if rev, ok := s.ledger.Revocations[grantIDKey(grant.GrantID)]; ok && !at.Before(rev.RevokedAt) {
		return capabilityError(CodeRevoked, "capability is revoked")
	}
	return nil
}

func (s *Service) state(grant identityapi.CapabilityGrant, at time.Time) string {
	if rev, ok := s.ledger.Revocations[grantIDKey(grant.GrantID)]; ok && !at.Before(rev.RevokedAt) {
		return "revoked"
	}
	if at.Before(grant.NotBefore) {
		return "not_yet_valid"
	}
	if !at.Before(grant.NotAfter) {
		return "expired"
	}
	return "active"
}

var _ identityapi.CapabilityResolver = (*Service)(nil)
var _ identityapi.CapabilitySenderAuthorizer = (*Service)(nil)
