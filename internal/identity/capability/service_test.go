package capability

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"
	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

var capabilityTestNow = time.Unix(1_800_000_000, 0).UTC()

func TestServiceImportsPersistsAndResolvesGrant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	storeKey := bytes.Repeat([]byte{0xa5}, 32)
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	service, err := NewService(path, storeKey, grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)

	ref, err := service.ImportGrant(grant)
	require.NoError(t, err)
	require.NotEmpty(t, ref)
	resolved, err := service.ResolveCapability(validUse(ref, grant))
	require.NoError(t, err)
	require.Equal(t, grant.Generation, resolved.Generation)
	require.Equal(t, grant.Scope, resolved.Scope)
	require.Equal(t, grant.Secret.Bytes(), resolved.Secret.Bytes())

	restored, err := NewService(path, storeKey, grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)
	_, err = restored.ResolveCapability(validUse(ref, grant))
	require.NoError(t, err)
}

func TestCapabilityStoreDoesNotContainRecoverableSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	storeKey := bytes.Repeat([]byte{0xa5}, 32)
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	service, err := NewService(path, storeKey, grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, nil)
	require.NoError(t, err)
	_, err = service.ImportGrant(grant)
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, raw, grant.Secret.Bytes())
	require.NotContains(t, string(raw), grant.SubjectPrincipal)
	_, err = NewService(path, bytes.Repeat([]byte{0xb6}, 32), grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, nil)
	require.ErrorContains(t, err, "authentication failed")
}

func TestServiceRejectsUnauthorizedCapabilityUse(t *testing.T) {
	service, grant, ref := importedTestService(t)
	tests := []struct {
		name string
		use  identityapi.CapabilityUse
		code string
	}{
		{name: "missing", use: identityapi.CapabilityUse{Ref: "cap_missing"}, code: CodeMissing},
		{name: "subject", use: withSubject(validUse(ref, grant), otherPrincipal()), code: CodeScopeDenied},
		{name: "scope", use: withScope(validUse(ref, grant), identityapi.CapabilityDataExchange), code: CodeScopeDenied},
		{name: "permission", use: withPermission(validUse(ref, grant), identityapi.CapabilityDelegate), code: CodeScopeDenied},
		{name: "early", use: withTime(validUse(ref, grant), grant.NotBefore.Add(-time.Second)), code: CodeNotYetValid},
		{name: "expired", use: withTime(validUse(ref, grant), grant.NotAfter), code: CodeExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.ResolveCapability(tt.use)
			requireCapabilityCode(t, err, tt.code)
		})
	}
}

func TestServiceRequiresAndEnforcesPolicyAdmission(t *testing.T) {
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	_, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32), grant.SubjectPrincipal, trustedIssuer(issuerPublic), nil, nil)
	require.ErrorContains(t, err, "policy admission is required")

	service, err := NewService(
		filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32),
		grant.SubjectPrincipal, trustedIssuer(issuerPublic), denyCapabilityAdmission{}, nil,
	)
	require.NoError(t, err)
	ref, err := service.ImportGrant(grant)
	require.NoError(t, err)
	_, err = service.ResolveCapability(validUse(ref, grant))
	requireCapabilityCode(t, err, CodeScopeDenied)
}

func TestServiceAppliesSignedRevocationAtEffectiveTime(t *testing.T) {
	service, grant, ref := importedTestService(t)
	_, _, issuerPrivate := signedTestGrant(t, 1)
	rev, err := SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: grant.GrantID, IssuerPrincipal: grant.IssuerPrincipal,
		RevokedAt: capabilityTestNow.Add(time.Minute),
	}, issuerPrivate)
	require.NoError(t, err)
	require.NoError(t, service.ApplyRevocation(rev))

	_, err = service.ResolveCapability(withTime(validUse(ref, grant), capabilityTestNow))
	require.NoError(t, err)
	_, err = service.ResolveCapability(withTime(validUse(ref, grant), rev.RevokedAt))
	requireCapabilityCode(t, err, CodeRevoked)
}

func TestServiceRejectsTamperedGrantSignature(t *testing.T) {
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	grant.Permissions |= identityapi.CapabilityDelegate
	service, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32), grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, nil)
	require.NoError(t, err)

	_, err = service.ImportGrant(grant)
	requireCapabilityCode(t, err, CodeInvalid)
}

func TestServiceRejectsGrantBoundToAnotherCanonicalPrincipal(t *testing.T) {
	grant, issuerPublic, issuerPrivate := signedTestGrant(t, 1)
	grant.SubjectPrincipal = otherPrincipal()
	grant, err := SignGrant(grant, issuerPrivate)
	require.NoError(t, err)
	service, err := NewService(
		filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32),
		subjectIdentityPrincipal(), trustedIssuer(issuerPublic), allowCapabilityAdmission{}, nil,
	)
	require.NoError(t, err)

	_, err = service.ImportGrant(grant)
	requireCapabilityCode(t, err, CodeScopeDenied)
}

func TestServiceRejectsGrantFromUntrustedIssuer(t *testing.T) {
	grant, _, _ := signedTestGrant(t, 1)
	service, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32), grant.SubjectPrincipal, nil, allowCapabilityAdmission{}, nil)
	require.NoError(t, err)

	_, err = service.ImportGrant(grant)
	requireCapabilityCode(t, err, CodeIssuerUntrusted)
}

func TestServiceAuthorizesAndRevokesImportedSenderGrant(t *testing.T) {
	localGrant, issuerPublic, issuerPrivate := signedTestGrant(t, 1)
	senderGrant := localGrant
	senderGrant.GrantID = fixedID(0x99)
	senderGrant.SubjectPrincipal = otherPrincipal()
	senderGrant, err := SignGrant(senderGrant, issuerPrivate)
	require.NoError(t, err)
	service, err := NewService(
		filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{1}, 32),
		localGrant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, nil,
	)
	require.NoError(t, err)
	require.NoError(t, service.ImportSenderGrant(senderGrant))
	use := identityapi.CapabilitySenderUse{
		GrantID: senderGrant.GrantID, ChannelID: senderGrant.ChannelID,
		Generation: senderGrant.Generation, Subject: senderGrant.SubjectPrincipal,
		Permission: identityapi.CapabilityPublish, Scope: senderGrant.Scope,
		At: capabilityTestNow, ObservedAt: capabilityTestNow,
	}
	require.NoError(t, service.AuthorizeCapabilitySender(use))
	wrongGeneration := use
	wrongGeneration.Generation++
	requireCapabilityCode(t, service.AuthorizeCapabilitySender(wrongGeneration), CodeScopeDenied)

	revocation, err := SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: senderGrant.GrantID,
		IssuerPrincipal: senderGrant.IssuerPrincipal, RevokedAt: capabilityTestNow,
	}, issuerPrivate)
	require.NoError(t, err)
	require.NoError(t, service.ApplyRevocation(revocation))
	requireCapabilityCode(t, service.AuthorizeCapabilitySender(use), CodeRevoked)
	backdated := use
	backdated.At = capabilityTestNow.Add(-time.Second)
	requireCapabilityCode(t, service.AuthorizeCapabilitySender(backdated), CodeRevoked)
}

func TestHPKEGrantDeliveryIsRecipientBoundAndPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ardents.db")
	storeKey := bytes.Repeat([]byte{0x55}, 32)
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	recipient, err := NewService(path, storeKey, grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)
	subjectPrivate := subjectIdentityPrivate(t, grant.SubjectPrincipal)
	attestation, err := recipient.AttestDeliveryPublicKey(subjectPrivate, capabilityTestNow.Add(24*time.Hour))
	require.NoError(t, err)
	ciphertext, err := SealGrantForRecipient(grant, attestation, capabilityTestNow)
	require.NoError(t, err)
	require.NotContains(t, ciphertext, grant.Secret.Bytes())

	ref, err := recipient.ReceiveDeliveredGrant(ciphertext)
	require.NoError(t, err)
	opened, err := recipient.ResolveCapability(validUse(ref, grant))
	require.NoError(t, err)
	require.Equal(t, grant.Secret.Bytes(), opened.Secret.Bytes())
	other, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), storeKey, otherPrincipal(), trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)
	_, err = other.ReceiveDeliveredGrant(ciphertext)
	require.ErrorContains(t, err, "authentication failed")

	restored, err := NewService(path, storeKey, grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)
	restoredAttestation, err := restored.AttestDeliveryPublicKey(subjectPrivate, capabilityTestNow.Add(24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, attestation.DeliveryPublicKey, restoredAttestation.DeliveryPublicKey)
	_, err = restored.ReceiveDeliveredGrant(ciphertext)
	require.NoError(t, err)

	dbRaw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, dbRaw, grant.Secret.Bytes())
}

func TestSealGrantRejectsTamperedOrExpiredDeliveryAttestation(t *testing.T) {
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	service, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{9}, 32), grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time { return capabilityTestNow })
	require.NoError(t, err)
	attestation, err := service.AttestDeliveryPublicKey(subjectIdentityPrivate(t, grant.SubjectPrincipal), capabilityTestNow.Add(time.Hour))
	require.NoError(t, err)

	tampered := attestation
	tampered.DeliveryPublicKey = append([]byte(nil), attestation.DeliveryPublicKey...)
	tampered.DeliveryPublicKey[0] ^= 1
	_, err = SealGrantForRecipient(grant, tampered, capabilityTestNow)
	require.ErrorContains(t, err, "signature is invalid")
	_, err = SealGrantForRecipient(grant, attestation, attestation.NotAfter)
	require.ErrorContains(t, err, "not currently valid")
}

func TestCapabilityFormattingRedactsSecretAndIdentifiers(t *testing.T) {
	grant, _, _ := signedTestGrant(t, 1)
	require.Equal(t, "capability-grant[redacted]", grant.String())
	require.NotContains(t, grant.String(), string(grant.Secret.Bytes()))
	secretText := grant.Secret.String()
	require.Equal(t, "[redacted]", secretText)
	raw, err := json.Marshal(grant)
	require.NoError(t, err)
	require.NotContains(t, string(raw), base64.StdEncoding.EncodeToString(grant.Secret.Bytes()))
	require.NotContains(t, string(raw), "ChannelID")
	require.NotContains(t, string(raw), "GrantID")

	service, importedGrant, ref := importedTestService(t)
	resolved, err := service.ResolveCapability(validUse(ref, importedGrant))
	require.NoError(t, err)
	resolvedRaw, err := json.Marshal(resolved)
	require.NoError(t, err)
	require.NotContains(t, string(resolvedRaw), string(ref))
	require.NotContains(t, string(resolvedRaw), "ChannelID")
	require.NotContains(t, string(resolvedRaw), "GrantID")
	require.NotContains(t, string(resolvedRaw), base64.StdEncoding.EncodeToString(importedGrant.Secret.Bytes()))

	statusesRaw, err := json.Marshal(service.Statuses(capabilityTestNow))
	require.NoError(t, err)
	require.Contains(t, string(statusesRaw), "active")
	require.NotContains(t, string(statusesRaw), string(ref))
	require.NotContains(t, string(statusesRaw), importedGrant.SubjectPrincipal)
}

func importedTestService(t *testing.T) (*Service, identityapi.CapabilityGrant, identityapi.CapabilityRef) {
	t.Helper()
	grant, issuerPublic, _ := signedTestGrant(t, 1)
	service, err := NewService(filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{7}, 32), grant.SubjectPrincipal, trustedIssuer(issuerPublic), allowCapabilityAdmission{}, func() time.Time {
		return capabilityTestNow
	})
	require.NoError(t, err)
	ref, err := service.ImportGrant(grant)
	require.NoError(t, err)
	return service, grant, ref
}

func signedTestGrant(t *testing.T, generation uint32) (identityapi.CapabilityGrant, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	seed := bytes.Repeat([]byte{0x42}, ed25519.SeedSize)
	issuerPrivate := ed25519.NewKeyFromSeed(seed)
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	subjectPrivate := subjectIdentityPrivate(t, "")
	subjectPublic := subjectPrivate.Public().(ed25519.PublicKey)
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{0x24}, 32))
	require.True(t, ok)
	grant := identityapi.CapabilityGrant{
		Version: 1, ChannelID: fixedID(0x11), Generation: generation,
		Secret: secret, GrantID: fixedID(byte(0x20 + generation)),
		IssuerPrincipal:  identityprincipal.DeriveID("p", issuerPublic),
		SubjectPrincipal: identityprincipal.DeriveID("p", subjectPublic), Scope: identityapi.CapabilityRealmDiscovery,
		Permissions: identityapi.CapabilitySubscribe | identityapi.CapabilityPublish,
		NotBefore:   capabilityTestNow.Add(-time.Hour), NotAfter: capabilityTestNow.Add(time.Hour),
	}
	signed, err := SignGrant(grant, issuerPrivate)
	require.NoError(t, err)
	return signed, issuerPublic, issuerPrivate
}

func fixedID(value byte) [16]byte {
	var id [16]byte
	for index := range id {
		id[index] = value
	}
	return id
}

func validUse(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant) identityapi.CapabilityUse {
	return identityapi.CapabilityUse{
		Ref: ref, Subject: grant.SubjectPrincipal, Scope: grant.Scope,
		Permission: identityapi.CapabilityPublish, At: capabilityTestNow,
	}
}

func withSubject(use identityapi.CapabilityUse, subject string) identityapi.CapabilityUse {
	use.Subject = subject
	return use
}

func withScope(use identityapi.CapabilityUse, scope identityapi.CapabilityScope) identityapi.CapabilityUse {
	use.Scope = scope
	return use
}

func withPermission(use identityapi.CapabilityUse, permission identityapi.CapabilityPermission) identityapi.CapabilityUse {
	use.Permission = permission
	return use
}

func withTime(use identityapi.CapabilityUse, at time.Time) identityapi.CapabilityUse {
	use.At = at
	return use
}

func requireCapabilityCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	capabilityErr, ok := errors.AsType[*Error](err)
	require.True(t, ok)
	require.Equal(t, code, capabilityErr.Code)
	require.NotContains(t, err.Error(), "p_subject")
}

func trustedIssuer(public ed25519.PublicKey) map[string]ed25519.PublicKey {
	return map[string]ed25519.PublicKey{
		identityprincipal.DeriveID("p", public): public,
	}
}

type allowCapabilityAdmission struct{}

func (allowCapabilityAdmission) AllowCapabilityUse(identityapi.CapabilityUse) error { return nil }

type denyCapabilityAdmission struct{}

func (denyCapabilityAdmission) AllowCapabilityUse(identityapi.CapabilityUse) error {
	return fmt.Errorf("operator policy detail must not escape")
}

func subjectIdentityPrivate(t *testing.T, expected string) ed25519.PrivateKey {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize))
	if expected != "" {
		require.Equal(t, expected, identityprincipal.DeriveID("p", private.Public().(ed25519.PublicKey)))
	}
	return private
}

func otherPrincipal() string {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x32}, ed25519.SeedSize))
	return identityprincipal.DeriveID("p", private.Public().(ed25519.PublicKey))
}

func subjectIdentityPrincipal() string {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize))
	return identityprincipal.DeriveID("p", private.Public().(ed25519.PublicKey))
}
