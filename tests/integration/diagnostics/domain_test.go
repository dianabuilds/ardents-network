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

	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics"
	"ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	db "ardents/internal/storage"
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
		Data:    runtimeinfra.DataConfig{Dir: dir},
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
		Data: runtimeinfra.DataConfig{Dir: dir},
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
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
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
		Data:    runtimeinfra.DataConfig{Dir: t.TempDir()},
		Privacy: privacy.Receiver,
	})

	remoteNode := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "diag-trust-remote",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"remote://bootstrap"}},
		Data:    runtimeinfra.DataConfig{Dir: t.TempDir()},
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
	discoveryStatus := localNode.GetDiscoveryStatus()
	require.Equal(t, 1, discoveryStatus.RemoteRecords)
	require.Equal(t, 1, discoveryStatus.RejectedRecords)
	peers := localNode.ListPeers()
	require.Len(t, peers, 1)
	require.Equal(t, "degraded", peers[0].State)
	require.False(t, peers[0].Trust.Trusted)
	require.False(t, peers[0].Trust.Usable)
	require.True(t, hasDiagnosticsEvent(snapshot,

		"trust", "catalog_untrusted",

		record.RecordID()), "expected trust diagnostics event for untrusted record")

}

func TestDiscoveryRestartFailsClosedForPersistedInvalidVersionedRecord(t *testing.T) {
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
	validRecord := snapshotNodeRecord(t, record)
	evidence := retainedDiscoveryEvidence(t, validRecord)
	record.Signature = "not-base64"
	entry := discovery.Entry{
		Record:   snapshotNodeRecord(t, record),
		Source:   discoveryrecord.Imported,
		SeenAt:   time.Now().UTC(),
		Evidence: evidence,
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"schema_version": 2,
			"records":        []discovery.Entry{entry},
			"state":          "ready",
		})
		require.NoErrorf(t, err, "save discovery records: %v", err)
	}

	restarted := discovery.NewInDir(dir)
	err := restarted.Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "persisted discovery snapshot is invalid")
	require.Contains(t, err.Error(), "record signature is invalid")

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
		Record:   snapshotNodeRecord(t, record),
		Source:   discoveryrecord.Imported,
		SeenAt:   now,
		Evidence: retainedDiscoveryEvidence(t, snapshotNodeRecord(t, record)),
	}
	{
		err := db.SaveJSON(filepath.Join(dir, "ardents.db"), "discovery", "records", map[string]any{
			"schema_version": 2,
			"records":        []discovery.Entry{entry},
			"state":          "ready",
		})
		require.NoErrorf(t, err, "save discovery records: %v", err)
	}

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "diag-trust-expiry-observed",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
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
		if !hasDiagnosticsEvent(snapshot, "trust", "catalog_degraded", record.RecordID()) {
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

func diagnosticsSnapshot(t *testing.T, runtime *runtimeprocess.Node) diagapi.DiagSnapshot {
	t.Helper()
	return testkit.Diagnostics(runtime).DiagnosticsSnapshot()
}

func pendingOperations(t *testing.T, runtime *runtimeprocess.Node) []diagapi.OperationSnapshot {
	t.Helper()
	return testkit.Diagnostics(runtime).PendingOperations()
}

func hasDiagnosticsEvent(snapshot diagapi.DiagSnapshot, domain string, eventType string, resource string) bool {
	for _, item := range snapshot.RecentEvents {
		if item.Domain == domain && item.Type == eventType && item.Resource == resource {
			return true
		}
	}
	return false
}

func signedNodeRecord(t *testing.T, endpoints []string) (discoveryapi.CatalogRecordSnapshot, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityprincipal.FromPublicKey(publicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	record := discoveryapi.CatalogRecordSnapshot{
		Version: discoveryrecord.Version,
		Node: &discoveryapi.CatalogNodeFactsSnapshot{
			Principal: principal,
			PublicKey: publicKey,
			Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	payload, err := discovery.Canonical(snapshotNodeRecord(t, record))
	require.NoErrorf(t, err, "canonical record: %v", err)

	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record, private
}

func signDiscoveryRecord(t *testing.T, record *discoveryapi.CatalogRecordSnapshot, private ed25519.PrivateKey) {
	t.Helper()

	payload, err := discovery.Canonical(snapshotNodeRecord(t, *record))
	require.NoErrorf(t, err, "canonical record: %v", err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
}

func snapshotNodeRecord(t *testing.T, snapshot discoveryapi.CatalogRecordSnapshot) discovery.Record {
	t.Helper()
	require.NotNil(t, snapshot.Node)
	require.Nil(t, snapshot.Service)
	principal, err := identityprincipal.Parse(snapshot.Node.Principal)
	require.NoError(t, err)
	return discovery.Record{
		Version: snapshot.Version,
		Node: &discoveryrecord.NodeFacts{
			Principal: principal,
			PublicKey: snapshot.Node.PublicKey,
			Endpoints: append([]string(nil), snapshot.Node.Endpoints...),
		},
		IssuedAt: snapshot.IssuedAt, ExpiresAt: snapshot.ExpiresAt, Signature: snapshot.Signature,
	}
}

func retainedDiscoveryEvidence(t *testing.T, record discovery.Record) discoveryrecord.VerificationEvidence {
	t.Helper()
	evidence, err := discovery.NewTrustEvaluator(nil).VerifyRetained(record)
	require.NoError(t, err)
	return evidence
}
