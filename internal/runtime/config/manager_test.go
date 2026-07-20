package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingApplier struct {
	active    Document
	failApply bool
	rollbacks int
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
	a.active = previous
	return nil
}

func TestManagerAppliesReloadablePolicyAndRedactsEffectiveSnapshot(t *testing.T) {
	doc := Defaults()
	doc.API.TokenFile = filepath.Join(t.TempDir(), "operator-secret-token")
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

	snapshot := manager.Snapshot()
	raw, err := json.Marshal(snapshot.Effective)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "operator-secret-token")
	require.NotContains(t, string(raw), "private-key")
	require.Contains(t, string(raw), `"token_file":"configured"`)
}

func TestManagerRedactsAllProtectedPrivacyReferences(t *testing.T) {
	doc := Defaults()
	doc.Privacy = PrivacyConfig{
		Required: true, CapabilityStore: "/protected/capabilities.db",
		CapabilityStoreKeyFile: "/protected/capabilities.key", ReplayKeyFile: "/protected/replay.key",
		Subject: "p_private_subject", TrustedIssuers: map[string]string{"p_issuer": "raw-public-key"},
		Discovery: PrivacyChannelConfig{Reference: "secret-discovery-ref", ReplayPath: "/protected/discovery.db"},
		Data:      PrivacyChannelConfig{Reference: "secret-data-ref", ReplayPath: "/protected/data.db"},
	}
	path := writeDocument(t, doc)
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	raw, err := json.Marshal(manager.Snapshot().Effective)
	require.NoError(t, err)
	for _, protected := range []string{
		"/protected/", "p_private_subject", "raw-public-key", "secret-discovery-ref", "secret-data-ref",
	} {
		require.NotContains(t, string(raw), protected)
	}
	require.Contains(t, string(raw), `"capability_store":"configured"`)
	require.Contains(t, string(raw), `"p_issuer":"configured"`)
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
	doc.API.TokenFile = "resolved-secret-reference"
	path := writeDocument(t, Defaults())
	manager, err := NewManager(path, doc)
	require.NoError(t, err)
	require.NoError(t, manager.RegisterResolver(func(candidate Document) (Document, error) {
		candidate.API.TokenFile = "resolved-secret-reference"
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
