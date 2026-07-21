package capability

import (
	"crypto/hmac"

	identityapi "ardents/internal/identity"
)

func (s *Service) ImportSenderGrant(grant identityapi.CapabilityGrant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	issuerPublic, ok := s.issuers[grant.IssuerPrincipal]
	if !ok {
		return capabilityError(CodeIssuerUntrusted, "capability issuer is not trusted")
	}
	if err := validateGrant(grant, issuerPublic); err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
	if err := s.validateSenderGrantConflict(grant); err != nil {
		return err
	}
	key := grantIDKey(grant.GrantID)
	if _, exists := s.ledger.SenderGrants[key]; exists {
		return nil
	}
	next := cloneLedger(s.ledger)
	next.SenderGrants[key] = persistGrant(grant)
	if err := s.store.save(next); err != nil {
		return err
	}
	s.ledger = next
	return nil
}

func (s *Service) AuthorizeCapabilitySender(use identityapi.CapabilitySenderUse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.ledger.SenderGrants[grantIDKey(use.GrantID)]
	if !ok {
		return capabilityError(CodeMissing, "sender capability grant not found")
	}
	grant, err := stored.restore()
	if err != nil {
		return capabilityError(CodeInvalid, err.Error())
	}
	if use.ChannelID != grant.ChannelID || use.Generation != grant.Generation {
		return capabilityError(CodeScopeDenied, "sender capability channel or generation denied")
	}
	capabilityUse := identityapi.CapabilityUse{
		Subject: use.Subject, Permission: use.Permission, Scope: use.Scope, At: use.At,
	}
	if err := s.authorize(grant, capabilityUse, use.At.UTC()); err != nil {
		return err
	}
	observedAt := use.ObservedAt.UTC()
	if observedAt.IsZero() {
		observedAt = s.clock().UTC()
	}
	if rev, ok := s.ledger.Revocations[grantIDKey(grant.GrantID)]; ok &&
		!observedAt.Before(rev.RevokedAt) {
		return capabilityError(CodeRevoked, "sender capability is revoked at receive time")
	}
	if err := s.admission.AllowCapabilityUse(capabilityUse); err != nil {
		return capabilityError(CodeScopeDenied, "sender capability use denied by policy")
	}
	return nil
}

func (s *Service) validateSenderGrantConflict(grant identityapi.CapabilityGrant) error {
	if rev, ok := s.ledger.Revocations[grantIDKey(grant.GrantID)]; ok &&
		rev.IssuerPrincipal != grant.IssuerPrincipal {
		return capabilityError(CodeInvalid, "revocation issuer does not match sender grant issuer")
	}
	current, ok := s.ledger.SenderGrants[grantIDKey(grant.GrantID)]
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
		return capabilityError(CodeInvalid, "sender grant identifier conflicts with retained grant")
	}
	return nil
}
