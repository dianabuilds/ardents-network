//go:build integration

package diagnostics_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics/api"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	identityapi "ardents/internal/identity/api"
	db "ardents/internal/persistence"
	runtimeinfra "ardents/internal/runtime/process"
	runtimeprocess "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDiagnosticsRestartKeepsRetainedExplainabilityWithoutMaskingActiveHealth(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	rec := diagnostics.NewInDir(dir)
	rec.SetPrimary(diagnostics.HealthFailed, &diagnostics.Reason{
		Code:                   "node.transport.start_failed",
		Domain:                 "transport",
		Summary:                "transport start failed",
		Detail:                 "bind failed",
		Recovery:               "restart_required",
		OperatorActionRequired: true,
		Resource:               "transport",
	})
	rec.SetSubsystem("diagnostics", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:                   "diagnostics.persistence.degraded",
		Domain:                 "diagnostics",
		Summary:                "diagnostics persistence degraded",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               "operations",
	})
	rec.RecordEvent("transport", "start_failed", "transport", "transport.start_failed", "node.transport.start_failed", map[string]any{
		"public": "value",
		"secret": "must-redact",
	})
	rec.BeginOperation("node.startup.workloads", "workload", "workloads", true, "restart node")

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "diag-restart",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:    runtimeinfra.NodeDataConfig{Dir: dir},
		Privacy: privacy.Receiver,
	})

	snapshot := diagnosticsSnapshot(t, n)
	require.Falsef(t, snapshot.Health.State !=
		"ready", "health state = %q, want ready after healthy restart", snapshot.Health.State)
	require.Nilf(t, snapshot.Health.PrimaryReason, "primary reason = %#v, want nil active reason after healthy restart", snapshot.Health.PrimaryReason)
	require.Falsef(t, len(snapshot.Health.Subsystems) != 0, "subsystems = %#v, want no retained overlay in active health", snapshot.Health.Subsystems)

	foundEvent := false
	for _, item := range snapshot.RecentEvents {
		if item.Domain == "transport" && item.Type == "start_failed" {
			foundEvent = true
			require.Falsef(t, item.Payload["secret"] !=
				"[redacted]", "payload secret = %#v, want redacted", item.Payload["secret"])

		}
	}
	require.True(t, foundEvent, "expected persisted diagnostics event after restart")

	pending := pendingOperations(t, n)
	require.Falsef(t, len(pending) != 1, "pending = %d, want 1", len(pending))
	require.Falsef(t, pending[0].State != "recovering", "state = %q, want recovering", pending[0].State)

}

func TestDiagnosticsRestartKeepsMalformedPendingOperationVisible(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	raw := []byte(`{
  "operations": [
    {
      "kind": "node.startup.workloads",
      "state": "broken-state",
      "domain": "workload",
      "resource": "workloads",
      "recoverable": true,
      "recovery_action": "restart node",
      "updated_at": "2026-03-20T10:05:00Z"
    }
  ]
}`)
	{
		err := os.WriteFile(filepath.Join(dir, "operations.json"), raw, 0o644)
		require.NoErrorf(t, err, "write operations ledger: %v", err)
	}

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "diag-ops-restart",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: dir},
	})

	snapshot := diagnosticsSnapshot(t, n)
	require.Falsef(t, len(snapshot.PendingOperations) != 1, "pending = %d, want 1", len(snapshot.PendingOperations))

	op := snapshot.PendingOperations[0]
	require.Falsef(t, op.State != "recovering", "state = %q, want recovering", op.State)
	require.Falsef(t, op.ID != "recovered-node-startup-workloads-workload-workloads-1774001100000000000", "id = %q, want deterministic recovered id", op.ID)
	require.Truef(t, strings.Contains(op.Reason,

		"invalid persisted operation state",
	), "reason = %q, want invalid persisted operation state", op.Reason)

}

func TestDiagnosticsSurfaceIncludesRecentEvents(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "diag-live",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
	})

	got := diagnosticsSnapshot(t, n)
	require.False(t, len(got.RecentEvents) ==
		0, "expected recent events")
	require.False(t, got.RecentEvents[len(got.
		RecentEvents)-1].Time.After(time.Now().UTC().
		Add(time.Second)), "unexpected event time")

}

func TestDiagnosticsProjectsUntrustedDiscoveryRecord(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	localNode := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "diag-trust-import",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:    runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		Privacy: privacy.Receiver,
	})

	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "diag-trust-remote",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Data:    runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		Privacy: privacy.Sender,
	})

	records, err := remoteNode.ListRecords()
	require.NoErrorf(t, err, "list remote records: %v", err)

	record := records[0]
	record.Source = "bootstrap"
	{
		_, err := localNode.ImportRecord(record)
		require.NoErrorf(t, err, "import record: %v", err)
	}

	snapshot := diagnosticsSnapshot(t, localNode)
	require.Falsef(t, snapshot.Health.State !=
		"ready", "health state = %q, want ready", snapshot.Health.State)

	reason := subsystemReason(snapshot, "trust")
	require.Nilf(t, reason, "trust subsystem reason = %#v, want nil advisory-only trust health", reason)

	trust := localNode.Snapshot().Trust
	require.Falsef(t, trust.State != "ready", "trust snapshot state = %q, want ready after local trust bootstrap", trust.State)
	require.Falsef(t, !trust.Usable || !trust.
		Trusted, "trust snapshot = %#v, want trusted usable local trust", trust)
	discovery := localNode.GetDiscoveryStatus()
	require.Equal(t, 1, discovery.RemoteRecords)
	require.Equal(t, 1, discovery.RejectedRecords)
	peers := localNode.ListPeers()
	require.Len(t, peers, 1)
	require.Equal(t, "degraded", peers[0].State)
	require.False(t, peers[0].Trust.Trusted)
	require.False(t, peers[0].Trust.Usable)
	require.True(t, hasDiagnosticsEvent(snapshot,

		"trust", "catalog_untrusted",

		record.ID), "expected trust diagnostics event for untrusted record")

}

func TestDiagnosticsProjectsPersistedInvalidDiscoveryRecordOnRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	record, _ := signedNodeRecord(t, []string{"tcp://invalid:9000"})
	record.Signature = "not-base64"
	entry := discovery.Entry{
		Record: discovery.Record{
			ID:        record.ID,
			Kind:      record.Kind,
			Subject:   record.Subject,
			Node:      record.Node,
			Device:    record.Device,
			Owner:     record.Owner,
			Service:   record.Service,
			Mode:      record.Mode,
			PublicKey: record.PublicKey,
			Endpoints: append([]string(nil), record.Endpoints...),
			IssuedAt:  record.IssuedAt,
			ExpiresAt: record.ExpiresAt,
			Signature: record.Signature,
		},
		Source: "cache",
		SeenAt: time.Now().UTC(),
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"records": []discovery.Entry{entry},
			"state":   "ready",
		})
		require.NoErrorf(t, err, "save discovery records: %v", err)
	}

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "diag-trust-restart",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: dir},
	})

	snapshot := diagnosticsSnapshot(t, n)
	reason := subsystemReason(snapshot, "trust")
	require.NotNil(t, reason, "expected trust subsystem reason after restart")
	require.Falsef(t, reason.Code != "trust.record.invalid", "reason code = %q, want trust.record.invalid", reason.Code)
	trust := n.Snapshot().Trust
	require.Equal(t, "degraded", trust.State)
	require.Equal(t, "unverified", trust.Outcome)
	require.False(t, trust.Valid)
	require.False(t, trust.Usable)
	require.Contains(t, trust.Reason, "signature is invalid")
	require.True(t, hasDiagnosticsEvent(snapshot,

		"trust", "catalog_degraded",

		record.ID), "expected invalid trust diagnostics event after restart")

}

func TestDiagnosticsObservedTruthProjectsExpiredDiscoveryRecordWithoutRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "diagnostics",
		ScenarioID:  "DII-001",
		Suite:       "integration",
		Tags:        []string{"integration", "diagnostics"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	now := time.Now().UTC()
	record, private := signedNodeRecord(t, []string{"tcp://expires-soon:9000"})
	record.ExpiresAt = now.Add(3 * time.Second)
	signDiscoveryRecord(t, &record, private)
	entry := discovery.Entry{
		Record: discovery.Record{
			ID:        record.ID,
			Kind:      record.Kind,
			Subject:   record.Subject,
			Node:      record.Node,
			Device:    record.Device,
			Owner:     record.Owner,
			Service:   record.Service,
			Mode:      record.Mode,
			PublicKey: record.PublicKey,
			Endpoints: append([]string(nil), record.Endpoints...),
			IssuedAt:  record.IssuedAt,
			ExpiresAt: record.ExpiresAt,
			Signature: record.Signature,
		},
		Source: "cache",
		SeenAt: now,
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"records": []discovery.Entry{entry},
			"state":   "ready",
		})
		require.NoErrorf(t, err, "save discovery records: %v", err)
	}

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "diag-trust-expiry-observed",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.NodeDataConfig{Dir: dir},
	})

	initial := diagnosticsSnapshot(t, n)
	require.Nil(t, subsystemReason(initial, "trust"))

	testkit.WaitForCondition(t, 5*time.Second, "trust diagnostics degrade after discovery record expiry", func() (bool, string) {
		snapshot := diagnosticsSnapshot(t, n)
		reason := subsystemReason(snapshot, "trust")
		if reason == nil {
			return false, "trust subsystem reason not projected yet"
		}
		if reason.Code != "trust.record.expired" {
			return false, "unexpected trust code: " + reason.Code
		}
		if !hasDiagnosticsEvent(snapshot, "trust", "catalog_degraded", record.ID) {
			return false, "catalog_degraded event for expired record not visible yet"
		}
		return true, ""
	})

	trust := n.Snapshot().Trust
	require.Equal(t, "degraded", trust.State)
	require.Equal(t, "expired", trust.Outcome)
	require.False(t, trust.Valid)
	require.False(t, trust.Usable)
	require.Equal(t, "record expired", trust.Reason)
}

func subsystemReason(snapshot diagapi.DiagSnapshot, domain string) *diagapi.ReasonSnapshot {
	for _, item := range snapshot.Health.Subsystems {
		if item.Domain == domain {
			return item.Reason
		}
	}
	return nil
}

func diagnosticsSnapshot(t *testing.T, runtime runtimeprocess.NodeRuntime) diagapi.DiagSnapshot {
	t.Helper()
	return runtime.DiagnosticsSnapshot()
}

func pendingOperations(t *testing.T, runtime runtimeprocess.NodeRuntime) []diagapi.OperationSnapshot {
	t.Helper()
	return runtime.PendingOperations()
}

func hasDiagnosticsEvent(snapshot diagapi.DiagSnapshot, domain string, eventType string, resource string) bool {
	for _, item := range snapshot.RecentEvents {
		if item.Domain == domain && item.Type == eventType && item.Resource == resource {
			return true
		}
	}
	return false
}

func signedNodeRecord(t *testing.T, endpoints []string) (discoveryapi.DiscoveryRecord, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityapi.PrincipalFromPublicKey(publicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	record := discoveryapi.DiscoveryRecord{
		ID:        principal + ":node",
		Kind:      "node",
		Subject:   principal,
		Node:      principal,
		Device:    "diag-test-device",
		PublicKey: publicKey,
		Endpoints: append([]string(nil), endpoints...),
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	payload, err := discovery.Canonical(discovery.Record{
		ID:        record.ID,
		Kind:      record.Kind,
		Subject:   record.Subject,
		Node:      record.Node,
		Device:    record.Device,
		Owner:     record.Owner,
		Service:   record.Service,
		Mode:      record.Mode,
		PublicKey: record.PublicKey,
		Endpoints: record.Endpoints,
	})
	require.NoErrorf(t, err, "canonical record: %v", err)

	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record, private
}

func signDiscoveryRecord(t *testing.T, record *discoveryapi.DiscoveryRecord, private ed25519.PrivateKey) {
	t.Helper()

	payload, err := discovery.Canonical(discovery.Record{
		ID:        record.ID,
		Kind:      record.Kind,
		Subject:   record.Subject,
		Node:      record.Node,
		Device:    record.Device,
		Owner:     record.Owner,
		Service:   record.Service,
		Mode:      record.Mode,
		PublicKey: record.PublicKey,
		Endpoints: append([]string(nil), record.Endpoints...),
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
	})
	require.NoErrorf(t, err, "canonical record: %v", err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
}
