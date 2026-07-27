package config

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

type recordingApplier struct {
	active       Document
	failApply    bool
	failRollback bool
	rollbacks    int
	commits      int
}

func (a *recordingApplier) Prepare(context.Context, Document, Document) error { return nil }

func (a *recordingApplier) Apply(_ context.Context, _ Document, next Document) error {
	if a.failApply {
		return errors.New("apply failed")
	}
	a.active = next
	return nil
}

func (a *recordingApplier) Rollback(_ context.Context, previous Document) error {
	a.rollbacks++
	if a.failRollback {
		return errors.New("rollback failed")
	}
	a.active = previous
	return nil
}

func (a *recordingApplier) Commit(context.Context) {
	a.commits++
}

func TestManagerAppliesReloadablePolicyAndRedactsEffectiveSnapshot(t *testing.T) {
	doc := Defaults()
	doc.Network.PrivateKeyPath = filepath.Join(t.TempDir(), "private-key")
	path := writeDocument(t, doc)
	applier := &recordingApplier{active: doc}
	manager, err := NewManager(path, doc, applier)
	require.NoError(t, err)

	doc.Policy.DisableServicePublication = true
	writeDocumentAt(t, path, doc)
	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeApplied, result.Outcome)
	require.Equal(t, uint64(2), result.ActiveGeneration)
	require.True(t, applier.active.Policy.DisableServicePublication)
	require.Equal(t, 1, applier.commits)

	snapshot := manager.Snapshot()
	raw, err := json.Marshal(snapshot.Effective)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private-key")
	require.Contains(t, string(raw), `"private_key_path":"configured"`)
}

func TestManagerRedactsAllProtectedPrivacyReferences(t *testing.T) {
	doc := Defaults()
	doc.Trust.Principals = []TrustedPrincipalConfig{trustedPrincipalConfig(t, "channel.issue")}
	doc.Privacy = PrivacyConfig{
		Required: true, ChannelGrantStore: "/protected/channel-grants.db",
		ChannelGrantStoreKeyFile: "/protected/channel-grants.key", ReplayKeyFile: "/protected/replay.key",
		Subject:   "p_private_subject",
		Discovery: PrivacyChannelConfig{Reference: "secret-discovery-ref", ReplayPath: "/protected/discovery.db"},
		Data:      PrivacyChannelConfig{Reference: "secret-data-ref", ReplayPath: "/protected/data.db"},
	}
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	raw, err := json.Marshal(manager.Snapshot().Effective)
	require.NoError(t, err)
	for _, protected := range []string{
		"/protected/", "p_private_subject", doc.Trust.Principals[0].PublicKey, "secret-discovery-ref", "secret-data-ref",
	} {
		require.NotContains(t, string(raw), protected)
	}
	require.Contains(t, string(raw), `"channel_grant_store":"configured"`)
	require.Contains(t, string(raw), `"public_key":"configured"`)
}

func TestManagerKeepsRestartCandidateSeparateFromActive(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	applier := &recordingApplier{active: doc}
	manager, err := NewManager(path, doc, applier)
	require.NoError(t, err)

	doc.Node.Profile = "constrained_light_client"
	doc.Network.ReachabilityMode = "outbound_only"
	writeDocumentAt(t, path, doc)
	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeRestartRequired, result.Outcome)
	require.Contains(t, result.RestartRequired, "node.profile")
	require.Equal(t, "service_node", applier.active.Node.Profile)
	require.Equal(t, uint64(1), result.ActiveGeneration)
}

func TestManagerReloadsDiscoveryOnlyTrustButKeepsOtherTrustRestartRequired(t *testing.T) {
	t.Run("unclassified trust change", func(t *testing.T) {
		doc := Defaults()
		path := writeDocument(t, doc)
		applier := &recordingApplier{active: doc}
		manager, err := NewManager(path, doc, applier)
		require.NoError(t, err)

		doc.Trust.Principals = []TrustedPrincipalConfig{managerTrustedPrincipal(
			t, identitytrust.PurposeDiscoveryPublish,
		)}
		writeDocumentAt(t, path, doc)
		result := manager.Reload(context.Background())

		require.Equal(t, OutcomeRestartRequired, result.Outcome)
		require.Empty(t, applier.active.Trust.Principals)
	})

	t.Run("discovery only", func(t *testing.T) {
		doc := Defaults()
		path := writeDocument(t, doc)
		applier := &recordingApplier{active: doc}
		manager, err := NewManager(path, doc, applier)
		require.NoError(t, err)
		require.NoError(t, manager.RegisterTrustChangeClassifier(func(_, candidate TrustConfig) bool {
			return len(candidate.Principals) == 1 &&
				len(candidate.Principals[0].Purposes) == 1 &&
				candidate.Principals[0].Purposes[0] == identitytrust.PurposeDiscoveryPublish
		}))

		doc.Trust.Principals = []TrustedPrincipalConfig{managerTrustedPrincipal(
			t, identitytrust.PurposeDiscoveryPublish,
		)}
		writeDocumentAt(t, path, doc)
		result := manager.Reload(context.Background())

		require.Equal(t, OutcomeApplied, result.Outcome)
		require.Equal(t, doc.Trust, applier.active.Trust)
	})

	t.Run("channel issuer", func(t *testing.T) {
		doc := Defaults()
		path := writeDocument(t, doc)
		applier := &recordingApplier{active: doc}
		manager, err := NewManager(path, doc, applier)
		require.NoError(t, err)

		doc.Trust.Principals = []TrustedPrincipalConfig{managerTrustedPrincipal(
			t, identitytrust.PurposeChannelIssue,
		)}
		writeDocumentAt(t, path, doc)
		result := manager.Reload(context.Background())

		require.Equal(t, OutcomeRestartRequired, result.Outcome)
		require.Contains(t, result.RestartRequired, "trust.principals")
		require.Empty(t, applier.active.Trust.Principals)
	})
}

func managerTrustedPrincipal(t *testing.T, purpose identitytrust.Purpose) TrustedPrincipalConfig {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	return TrustedPrincipalConfig{
		Principal: principal.String(),
		PublicKey: base64.StdEncoding.EncodeToString(public),
		Purposes:  []identitytrust.Purpose{purpose},
	}
}

func TestManagerClearsPendingRestartWhenSourceReturnsToActiveConfiguration(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	candidate := doc
	candidate.Network.ListenPort = 24001
	writeDocumentAt(t, path, candidate)
	require.Equal(t, OutcomeRestartRequired, manager.Reload(context.Background()).Outcome)
	require.NotEmpty(t, manager.Snapshot().PendingRestart)

	writeDocumentAt(t, path, doc)
	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeUnchanged, result.Outcome)
	require.Empty(t, manager.Snapshot().PendingRestart)
	require.Equal(t, uint64(1), result.ActiveGeneration)
	require.Equal(t, uint64(3), result.CandidateGeneration)
}

func TestManagerRejectsImmutableAndInvalidCandidates(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)

	immutable := doc
	immutable.Node.Name = "other"
	writeDocumentAt(t, path, immutable)
	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeRejectedImmutable, result.Outcome)
	require.Contains(t, result.Immutable, "node.name")

	require.NoError(t, os.WriteFile(path, []byte(`{"api_version":"broken"}`), 0o600))
	result = manager.Reload(context.Background())
	require.Equal(t, OutcomeRejectedInvalid, result.Outcome)
	require.Equal(t, uint64(1), result.ActiveGeneration)
}

func TestManagerRedactsSourcePathFromReloadFailure(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeRejectedInvalid, result.Outcome)
	require.Equal(t, "operator configuration source is unavailable", result.Reason)
	require.NotContains(t, result.Reason, path)
}

func TestManagerRejectsLegacyChannelGrantFieldWithoutChangingActiveConfig(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`{
		"api_version":"ardents.config/v1",
		"policy":{"disable_private_capability_use":true}
	}`), 0o600))

	result := manager.Reload(context.Background())

	require.Equal(t, OutcomeRejectedInvalid, result.Outcome)
	require.Equal(t, uint64(1), result.ActiveGeneration)
	require.Equal(t, uint64(1), result.CandidateGeneration)
	require.False(t, manager.active.Policy.DisablePrivateChannelGrantUse)
}

func TestManagerRejectsCandidateThatFailsRuntimeValidation(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	require.NoError(t, manager.RegisterValidator(func(candidate Document) error {
		if candidate.Network.StorePath == "unavailable" {
			return errors.New("runtime resource is unavailable")
		}
		return nil
	}))
	doc.Network.StorePath = "unavailable"
	writeDocumentAt(t, path, doc)

	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeRejectedInvalid, result.Outcome)
	require.Equal(t, uint64(1), result.ActiveGeneration)
	require.Equal(t, uint64(1), result.CandidateGeneration)
}

func TestManagerResolvesCandidateBeforeComparingAndValidating(t *testing.T) {
	doc := Defaults()
	doc.Network.StorePath = "resolved-store"
	path := writeDocument(t, Defaults())
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	require.NoError(t, manager.RegisterResolver(func(candidate Document) (Document, error) {
		candidate.Network.StorePath = "resolved-store"
		return candidate, nil
	}))
	require.Equal(t, OutcomeUnchanged, manager.Reload(context.Background()).Outcome)
}

func TestManagerRollsBackWhenAnyApplierFails(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	first := &recordingApplier{active: doc}
	second := &recordingApplier{active: doc, failApply: true}
	manager, err := NewManager(path, doc, first, second)
	require.NoError(t, err)

	doc.Policy.DisableServicePublication = true
	writeDocumentAt(t, path, doc)
	result := manager.Reload(context.Background())
	require.Equal(t, OutcomeRolledBack, result.Outcome)
	require.False(t, first.active.Policy.DisableServicePublication)
	require.Equal(t, 1, first.rollbacks)
	require.Zero(t, first.commits)
	require.Zero(t, second.commits)
	require.Equal(t, uint64(1), result.ActiveGeneration)
}

func TestManagerReportsRollbackFailureWithoutClaimingRestoration(t *testing.T) {
	doc := Defaults()
	path := writeDocument(t, doc)
	first := &recordingApplier{active: doc, failRollback: true}
	second := &recordingApplier{active: doc, failApply: true}
	manager, err := NewManager(path, doc, first, second)
	require.NoError(t, err)

	doc.Policy.DisableServicePublication = true
	writeDocumentAt(t, path, doc)
	result := manager.Reload(context.Background())

	require.Equal(t, OutcomeRollbackFailed, result.Outcome)
	require.Contains(t, result.Reason, "rollback applier 0")
	require.True(t, first.active.Policy.DisableServicePublication)
	require.Equal(t, uint64(1), result.ActiveGeneration)
}

func writeDocument(t *testing.T, doc Document) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ardents.json")
	writeDocumentAt(t, path, doc)
	return path
}

func writeDocumentAt(t *testing.T, path string, doc Document) {
	t.Helper()
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
}
