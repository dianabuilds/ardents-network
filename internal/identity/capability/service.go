package capability

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	identityapi "ardents/internal/identity"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
)

type Service struct {
	mu                    sync.Mutex
	store                 *Store
	ledger                ledger
	refKey                [32]byte
	clock                 func() time.Time
	trust                 *identitytrust.Registry
	localPrincipal        string
	admission             identityapi.CapabilityAdmission
	installAfterCommit    func() error
	activationAfterCommit func() error
}

type Status struct {
	Scope      identityapi.CapabilityScope
	Generation uint32
	State      string
	NotAfter   time.Time
}

func NewService(path string, storeKey []byte, localPrincipal string, trust *identitytrust.Registry, admission identityapi.CapabilityAdmission, clock func() time.Time) (*Service, error) {
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
	if trust == nil {
		trust, _ = identitytrust.NewRegistry(nil)
	}
	service := &Service{
		store: store, ledger: stored, clock: clock,
		trust: trust, localPrincipal: localPrincipal, admission: admission,
	}
	refKey, err := hkdf.Key(sha256.New, storeKey, nil, "ardents-capability-local-reference/1", len(service.refKey))
	if err != nil {
		return nil, err
	}
	copy(service.refKey[:], refKey)
	if err := service.restoreIssuerTransitions(); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *Service) ImportGrant(grant identityapi.CapabilityGrant) (identityapi.CapabilityRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	issuerPublic, ok := s.trustedIssuer(grant.IssuerPrincipal)
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
	issuerPublic, ok := s.trustedIssuer(rev.IssuerPrincipal)
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

// ReceiverGrantSnapshot returns a verified, detached view of grants retained
// for the local receiver. It is intentionally read-only and is used by the
// stopped local-v2 migration adapter; callers never receive store internals.
func (s *Service) ReceiverGrantSnapshot() ([]identityapi.CapabilityGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	refs := make([]string, 0, len(s.ledger.Grants))
	for ref := range s.ledger.Grants {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	grants := make([]identityapi.CapabilityGrant, 0, len(refs))
	for _, ref := range refs {
		grant, err := s.ledger.Grants[ref].restore()
		if err != nil || grant.SubjectPrincipal != s.localPrincipal {
			return nil, capabilityError(CodeInvalid, "capability store contains an invalid receiver grant")
		}
		issuer, ok := s.trustedIssuer(grant.IssuerPrincipal)
		if !ok || validateGrant(grant, issuer) != nil {
			return nil, capabilityError(CodeInvalid, "capability store contains an unverifiable receiver grant")
		}
		grant.Signature = append([]byte(nil), grant.Signature...)
		grants = append(grants, grant)
	}
	return grants, nil
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
	maps.Copy(out.Grants, source.Grants)
	maps.Copy(out.SenderGrants, source.SenderGrants)
	maps.Copy(out.Revocations, source.Revocations)
	maps.Copy(out.InstalledDeliveries, source.InstalledDeliveries)
	for key, pending := range source.PendingGenerations {
		pending.SenderGrants = append([]persistedGrant(nil), pending.SenderGrants...)
		pending.Revocations = append([]persistedRevocation(nil), pending.Revocations...)
		pending.ReceiptKey = append([]byte(nil), pending.ReceiptKey...)
		out.PendingGenerations[key] = pending
	}
	maps.Copy(out.PreviousGenerations, source.PreviousGenerations)
	for key, activated := range source.ActivatedGenerations {
		activated.Activation.Signature = append([]byte(nil), activated.Activation.Signature...)
		activated.Receipt.MAC = append([]byte(nil), activated.Receipt.MAC...)
		out.ActivatedGenerations[key] = activated
	}
	for key, activated := range source.ActivatedOperations {
		activated.Activation.Signature = append([]byte(nil), activated.Activation.Signature...)
		activated.Receipt.MAC = append([]byte(nil), activated.Receipt.MAC...)
		out.ActivatedOperations[key] = activated
	}
	out.IssuerTransitions = make(
		[]persistedIssuerTransition, len(source.IssuerTransitions),
	)
	for index, transition := range source.IssuerTransitions {
		transition.FromPublic = append([]byte(nil), transition.FromPublic...)
		transition.ToPublic = append([]byte(nil), transition.ToPublic...)
		out.IssuerTransitions[index] = transition
	}
	return out
}

func (s *Service) ReplaceTrustRegistry(registry *identitytrust.Registry) {
	if registry == nil {
		registry, _ = identitytrust.NewRegistry(nil)
	}
	s.mu.Lock()
	s.trust = registry
	s.mu.Unlock()
}

// AdoptChannelIssuerTransition atomically extends channel-issuer trust from an
// already trusted predecessor to its successor. The authority owner must
// validate the dual-signed transition proof before calling this boundary.
func (s *Service) AdoptChannelIssuerTransition(
	fromPrincipal string,
	fromPublic ed25519.PublicKey,
	toPrincipal string,
	toPublic ed25519.PublicKey,
) error {
	fromID, err := identityprincipal.Parse(fromPrincipal)
	if err != nil {
		return capabilityError(CodeInvalid, "authority predecessor is invalid")
	}
	toID, err := identityprincipal.Parse(toPrincipal)
	if err != nil || fromID.Equal(toID) {
		return capabilityError(CodeInvalid, "authority successor is invalid")
	}
	derived, err := identityprincipal.FromEd25519PublicKey(toPublic)
	if err != nil || !derived.Equal(toID) {
		return capabilityError(CodeInvalid, "authority successor key does not match")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.trust.Lookup(identitytrust.PurposeChannelIssue, fromID)
	if !ok || !current.Equal(fromPublic) {
		return capabilityError(CodeIssuerUntrusted, "authority predecessor is not trusted")
	}
	record := persistedIssuerTransition{
		FromPrincipal: fromPrincipal, FromPublic: append([]byte(nil), fromPublic...),
		ToPrincipal: toPrincipal, ToPublic: append([]byte(nil), toPublic...),
	}
	for _, retained := range s.ledger.IssuerTransitions {
		if retained.FromPrincipal == fromPrincipal &&
			retained.ToPrincipal == toPrincipal &&
			ed25519.PublicKey(retained.FromPublic).Equal(fromPublic) &&
			ed25519.PublicKey(retained.ToPublic).Equal(toPublic) {
			return nil
		}
	}
	nextTrust, err := applyIssuerTransitionTrust(s.trust, record)
	if err != nil {
		return capabilityError(CodeInvalid, "authority successor trust is invalid")
	}
	nextLedger := cloneLedger(s.ledger)
	nextLedger.IssuerTransitions = append(nextLedger.IssuerTransitions, record)
	if err := s.store.save(nextLedger); err != nil {
		return err
	}
	s.ledger, s.trust = nextLedger, nextTrust
	return nil
}

// FinalizeChannelIssuerTransition durably retires the predecessor's channel
// issuance purpose after authority truth proves every channel was rotated.
func (s *Service) FinalizeChannelIssuerTransition(
	fromPrincipal, toPrincipal string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := -1
	for candidate := range s.ledger.IssuerTransitions {
		record := s.ledger.IssuerTransitions[candidate]
		if record.FromPrincipal == fromPrincipal && record.ToPrincipal == toPrincipal {
			index = candidate
			break
		}
	}
	if index < 0 {
		return capabilityError(CodeMissing, "authority transition is not adopted")
	}
	if s.ledger.IssuerTransitions[index].Completed {
		return nil
	}
	nextTrust, err := retireIssuerTrust(s.trust, fromPrincipal, toPrincipal)
	if err != nil {
		return capabilityError(CodeInvalid, "authority transition trust cannot be finalized")
	}
	nextLedger := cloneLedger(s.ledger)
	nextLedger.IssuerTransitions[index].Completed = true
	if err := s.store.save(nextLedger); err != nil {
		return err
	}
	s.ledger, s.trust = nextLedger, nextTrust
	return nil
}

func (s *Service) restoreIssuerTransitions() error {
	trust := s.trust
	for _, transition := range s.ledger.IssuerTransitions {
		next, err := applyIssuerTransitionTrust(trust, transition)
		if err != nil {
			return capabilityError(CodeInvalid, "persisted authority transition is invalid")
		}
		trust = next
		if transition.Completed {
			trust, err = retireIssuerTrust(
				trust, transition.FromPrincipal, transition.ToPrincipal,
			)
			if err != nil {
				return capabilityError(CodeInvalid, "persisted authority transition completion is invalid")
			}
		}
	}
	s.trust = trust
	return nil
}

func applyIssuerTransitionTrust(
	current *identitytrust.Registry,
	transition persistedIssuerTransition,
) (*identitytrust.Registry, error) {
	fromID, err := identityprincipal.Parse(transition.FromPrincipal)
	if err != nil {
		return nil, err
	}
	fromPublic, ok := current.Lookup(identitytrust.PurposeChannelIssue, fromID)
	toID, err := identityprincipal.Parse(transition.ToPrincipal)
	if err != nil {
		return nil, err
	}
	derived, err := identityprincipal.FromEd25519PublicKey(
		ed25519.PublicKey(transition.ToPublic),
	)
	if err != nil || !derived.Equal(toID) || derived.Equal(fromID) {
		return nil, fmt.Errorf("authority successor is invalid")
	}
	toPublic, toTrusted := current.Lookup(identitytrust.PurposeChannelIssue, toID)
	predecessorTrusted := ok &&
		fromPublic.Equal(ed25519.PublicKey(transition.FromPublic))
	successorTrusted := toTrusted &&
		toPublic.Equal(ed25519.PublicKey(transition.ToPublic))
	if !predecessorTrusted && !(transition.Completed && successorTrusted) {
		return nil, fmt.Errorf("authority predecessor is not trusted")
	}
	snapshot := current.Snapshot()
	found := false
	for index := range snapshot.Entries {
		entry := &snapshot.Entries[index]
		if entry.Principal != transition.ToPrincipal {
			continue
		}
		if !entry.PublicKey.Equal(ed25519.PublicKey(transition.ToPublic)) {
			return nil, fmt.Errorf("authority successor conflicts")
		}
		found = true
		if !containsTrustPurpose(entry.Purposes, identitytrust.PurposeChannelIssue) {
			entry.Purposes = append(entry.Purposes, identitytrust.PurposeChannelIssue)
		}
	}
	if !found {
		snapshot.Entries = append(snapshot.Entries, identitytrust.Entry{
			Principal: transition.ToPrincipal,
			PublicKey: append(ed25519.PublicKey(nil), transition.ToPublic...),
			Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
		})
	}
	return identitytrust.NewRegistry(snapshot.Entries)
}

func retireIssuerTrust(
	current *identitytrust.Registry,
	fromPrincipal, toPrincipal string,
) (*identitytrust.Registry, error) {
	snapshot := current.Snapshot()
	successor := false
	for index := range snapshot.Entries {
		entry := &snapshot.Entries[index]
		if entry.Principal == toPrincipal {
			successor = containsTrustPurpose(
				entry.Purposes, identitytrust.PurposeChannelIssue,
			)
		}
		if entry.Principal == fromPrincipal {
			purposes := entry.Purposes[:0]
			for _, purpose := range entry.Purposes {
				if purpose != identitytrust.PurposeChannelIssue {
					purposes = append(purposes, purpose)
				}
			}
			entry.Purposes = purposes
		}
	}
	if !successor {
		return nil, fmt.Errorf("authority successor is not trusted")
	}
	entries := snapshot.Entries[:0]
	for _, entry := range snapshot.Entries {
		if len(entry.Purposes) > 0 {
			entries = append(entries, entry)
		}
	}
	return identitytrust.NewRegistry(entries)
}

func containsTrustPurpose(
	purposes []identitytrust.Purpose,
	target identitytrust.Purpose,
) bool {
	for _, purpose := range purposes {
		if purpose == target {
			return true
		}
	}
	return false
}

func (s *Service) trustedIssuer(raw string) (ed25519.PublicKey, bool) {
	principalID, err := identityprincipal.Parse(raw)
	if err != nil {
		return nil, false
	}
	return s.trust.Lookup(identitytrust.PurposeChannelIssue, principalID)
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
	want, err := canonicalGrant(existing)
	if err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
	got, err := canonicalGrant(grant)
	if err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
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
	if _, ok := s.trustedIssuer(grant.IssuerPrincipal); !ok {
		return capabilityError(CodeIssuerUntrusted, "capability issuer is not trusted")
	}
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
	if _, ok := s.trustedIssuer(grant.IssuerPrincipal); !ok {
		return "issuer_untrusted"
	}
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
