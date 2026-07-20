//go:build integration

package networkfoundation_test

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity/api"
	identitycapability "ardents/internal/identity/capability"
	identityprincipal "ardents/internal/identity/principal"
	networkprivacy "ardents/internal/network/privacy"
	policy "ardents/internal/policy"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPrivateCapabilitySelectorsInteroperateAndRevokeAcrossNodes(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NPI-001",
		Suite: "integration", Tags: []string{"integration", "network", "privacy", "capability"},
		Speed: "default", Environment: "local",
	})
	now := time.Unix(1_800_000_000, 0).UTC()
	issuerPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	issuer := identityprincipal.DeriveID("p", issuerPublic)
	trusted := map[string]ed25519.PublicKey{issuer: issuerPublic}
	grant := signedIntegrationGrant(t, issuerPrivate, issuer, now, 1, 0x21)

	left, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{0x51}, 32), grant.SubjectPrincipal, trusted, policy.New(policy.Config{}), func() time.Time { return now },
	)
	require.NoError(t, err)
	right, err := identitycapability.NewService(
		filepath.Join(t.TempDir(), "ardents.db"), bytes.Repeat([]byte{0x52}, 32), grant.SubjectPrincipal, trusted, policy.New(policy.Config{}), func() time.Time { return now },
	)
	require.NoError(t, err)
	leftRef, err := left.ImportGrant(grant)
	require.NoError(t, err)
	rightRef, err := right.ImportGrant(grant)
	require.NoError(t, err)
	require.NotEqual(t, leftRef, rightRef, "local references must not correlate across nodes")

	leftMaterial := resolvedMaterial(t, left, leftRef, grant, now)
	rightMaterial := resolvedMaterial(t, right, rightRef, grant, now)
	require.Equal(t, leftMaterial.ContentTopic, rightMaterial.ContentTopic)
	require.Equal(t, leftMaterial.EnvelopeKey(), rightMaterial.EnvelopeKey())
	require.NotContains(t, leftMaterial.ContentTopic, grant.SubjectPrincipal)
	require.NotContains(t, leftMaterial.ContentTopic, string(grant.Scope))

	rev, err := identitycapability.SignRevocation(identityapi.CapabilityRevocation{
		Version: 1, GrantID: grant.GrantID, IssuerPrincipal: issuer, RevokedAt: now,
	}, issuerPrivate)
	require.NoError(t, err)
	require.NoError(t, left.ApplyRevocation(rev))
	_, err = left.ResolveCapability(capabilityUse(leftRef, grant, now))
	require.ErrorContains(t, err, identitycapability.CodeRevoked)

	rotated := signedIntegrationGrant(t, issuerPrivate, issuer, now, 2, 0x62)
	rotatedRef, err := left.ImportGrant(rotated)
	require.NoError(t, err)
	rotatedMaterial := resolvedMaterial(t, left, rotatedRef, rotated, now)
	require.NotEqual(t, leftMaterial.ContentTopic, rotatedMaterial.ContentTopic)
	require.NotEqual(t, leftMaterial.EnvelopeKey(), rotatedMaterial.EnvelopeKey())
}

func signedIntegrationGrant(t *testing.T, issuerPrivate ed25519.PrivateKey, issuer string, now time.Time, generation uint32, secretByte byte) identityapi.CapabilityGrant {
	t.Helper()
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{secretByte}, 32))
	require.True(t, ok)
	subjectPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x43}, ed25519.SeedSize))
	subject := identityprincipal.DeriveID("p", subjectPrivate.Public().(ed25519.PublicKey))
	grant := identityapi.CapabilityGrant{
		Version: 1, ChannelID: integrationID(0x11), Generation: generation,
		Secret: secret, GrantID: integrationID(byte(0x30 + generation)),
		IssuerPrincipal: issuer, SubjectPrincipal: subject,
		Permissions: identityapi.CapabilitySubscribe | identityapi.CapabilityPublish,
		Scope:       identityapi.CapabilityRealmDiscovery,
		NotBefore:   now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}
	signed, err := identitycapability.SignGrant(grant, issuerPrivate)
	require.NoError(t, err)
	return signed
}

func resolvedMaterial(t *testing.T, service *identitycapability.Service, ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant, now time.Time) networkprivacy.Material {
	t.Helper()
	resolved, err := service.ResolveCapability(capabilityUse(ref, grant, now))
	require.NoError(t, err)
	material, err := networkprivacy.Derive(resolved)
	require.NoError(t, err)
	return material
}

func capabilityUse(ref identityapi.CapabilityRef, grant identityapi.CapabilityGrant, now time.Time) identityapi.CapabilityUse {
	return identityapi.CapabilityUse{
		Ref: ref, Subject: grant.SubjectPrincipal, Permission: identityapi.CapabilityPublish,
		Scope: grant.Scope, At: now,
	}
}

func integrationID(value byte) [16]byte {
	var id [16]byte
	for index := range id {
		id[index] = value
	}
	return id
}
