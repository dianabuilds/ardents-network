package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecorderMarksOpenOperationsRecoveringOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	raw := []byte(`{"operations":[{"id":"op-1","kind":"node.startup.workloads","state":"running","domain":"workload","resource":"workloads","recoverable":true,"recovery_action":"restart node","started_at":"2026-03-20T10:00:00Z","updated_at":"2026-03-20T10:00:00Z"}]}`)
	{
		err := os.WriteFile(path, raw, 0o644)
		require.NoErrorf(t, err, "write ledger: %v", err)
	}

	rec := New(path)
	{
		err := rec.Load()
		require.NoErrorf(t, err, "load ledger: %v", err)
	}

	pending := rec.PendingOperations()
	require.Falsef(t, len(pending) != 1, "pending = %d, want 1", len(pending))
	require.Falsef(t, pending[0].State != OperationRecovering, "state = %q, want recovering", pending[0].State)
}

func TestRecorderReturnsFatalCorruptLedgerForInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	{
		err := os.WriteFile(path, []byte("{invalid"), 0o644)
		require.NoErrorf(t, err, "write ledger: %v", err)
	}

	rec := New(path)
	err := rec.Load()
	require.Error(t, err, "expected corrupt ledger error")

	corrupt, ok := IsCorruptLedger(err)
	require.Truef(t, ok, "expected corrupt ledger error, got %T", err)
	require.True(t, corrupt.Fatal, "expected fatal corrupt ledger error")
}

func TestRecorderSnapshotIncludesHealthAndPendingOperations(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "operations.json"))
	rec.SetSubsystem("boot", HealthDegraded, &Reason{
		Code:                   "boot.join.degraded",
		Domain:                 "boot",
		Summary:                "bootstrap degraded",
		Impact:                 "join incomplete",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               "bootstrap",
	})
	rec.RecordEvent("node", "degraded", "node", "node.degraded", "boot.join.degraded", map[string]any{"state": "degraded"})
	rec.BeginOperation("node.startup.workloads", "workload", "workloads", true, "restart node")

	snapshot := rec.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthDegraded, "health = %q, want degraded", snapshot.Health.State)
	require.Falsef(t, snapshot.Health.PrimaryReason == nil || snapshot.Health.PrimaryReason.Code != "boot.join.degraded", "primary reason = %#v, want boot.join.degraded", snapshot.Health.PrimaryReason)
	require.Falsef(t, len(snapshot.RecentEvents) != 1, "events = %d, want 1", len(snapshot.RecentEvents))
	require.Falsef(t, len(snapshot.PendingOperations) != 1, "pending = %d, want 1", len(snapshot.PendingOperations))
}

func TestRecorderLoadRestoresHealthReasonsAndRecentEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	first := New(path)
	first.SetPrimary(HealthFailed, &Reason{
		Code:                   "node.transport.start_failed",
		Domain:                 "transport",
		Summary:                "transport start failed",
		Detail:                 "listen failed",
		Recovery:               "restart_required",
		OperatorActionRequired: true,
		Resource:               "transport",
	})
	first.SetSubsystem("diagnostics", HealthDegraded, &Reason{
		Code:                   "diagnostics.persistence.degraded",
		Domain:                 "diagnostics",
		Summary:                "diagnostics persistence degraded",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               "operations",
	})
	first.RecordEvent("transport", "start_failed", "transport", "transport.start_failed", "node.transport.start_failed", map[string]any{
		"public": "value",
		"secret": "redact-me",
	})
	first.BeginOperation("node.startup.transport", "transport", "transport", true, "restart node")

	second := New(path)
	{
		err := second.Load()
		require.NoErrorf(t, err, "load ledger: %v", err)
	}

	snapshot := second.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthFailed, "health = %q, want failed", snapshot.Health.State)
	require.Falsef(t, snapshot.Health.PrimaryReason == nil || snapshot.Health.PrimaryReason.Code != "node.transport.start_failed", "primary reason = %#v, want node.transport.start_failed", snapshot.Health.PrimaryReason)

	foundSubsystem := false
	for _, item := range snapshot.Health.Subsystems {
		if item.Domain != "diagnostics" {
			continue
		}
		foundSubsystem = true
		require.Falsef(t, item.Reason == nil || item.Reason.Code != "diagnostics.persistence.degraded", "subsystem reason = %#v, want diagnostics.persistence.degraded", item.Reason)
	}
	require.True(t, foundSubsystem, "expected diagnostics subsystem after load")
	require.Falsef(t, len(snapshot.RecentEvents) != 1, "events = %d, want 1", len(snapshot.RecentEvents))
	require.Falsef(t, snapshot.RecentEvents[0].ReasonCode != "node.transport.start_failed", "event reason = %q, want node.transport.start_failed", snapshot.RecentEvents[0].ReasonCode)
	require.Falsef(t, snapshot.RecentEvents[0].Payload["secret"] != "[redacted]", "event payload secret = %#v, want redacted", snapshot.RecentEvents[0].Payload["secret"])
	require.Falsef(t, len(snapshot.PendingOperations) != 1, "pending = %d, want 1", len(snapshot.PendingOperations))
	require.Falsef(t, snapshot.PendingOperations[0].State != OperationRecovering, "pending state = %q, want recovering", snapshot.PendingOperations[0].State)
}

func TestRecorderRetainedHealthDoesNotMaskActiveReadySnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	rec := New(path)
	rec.SetPrimary(HealthFailed, &Reason{
		Code:                   "node.transport.start_failed",
		Domain:                 "transport",
		Summary:                "transport start failed",
		Recovery:               "restart_required",
		OperatorActionRequired: true,
	})
	rec.RecordEvent("transport", "start_failed", "transport", "transport.start_failed", "node.transport.start_failed", nil)

	rec.RetainCurrentHealth()
	snapshot := rec.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthReady, "health = %q, want ready", snapshot.Health.State)
	require.Nilf(t, snapshot.Health.PrimaryReason, "primary reason = %#v, want nil active reason", snapshot.Health.PrimaryReason)
	require.Falsef(t, len(snapshot.Health.Subsystems) != 0, "subsystems = %#v, want no active retained overlay", snapshot.Health.Subsystems)
	require.Falsef(t, len(snapshot.RecentEvents) != 1, "events = %d, want retained event visibility", len(snapshot.RecentEvents))
}

func TestRecorderLoadKeepsMalformedOpenOperationVisibleAsRecovering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	updatedAt := time.Date(2026, 3, 20, 10, 5, 0, 0, time.UTC)
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
		err := os.WriteFile(path, raw, 0o644)
		require.NoErrorf(t, err, "write ledger: %v", err)
	}

	rec := New(path)
	err := rec.Load()
	require.Error(t, err, "expected non-fatal corrupt ledger error")

	corrupt, ok := IsCorruptLedger(err)
	require.Falsef(t, !ok || corrupt.Fatal, "error = %v, want non-fatal corrupt ledger", err)

	pending := rec.PendingOperations()
	require.Falsef(t, len(pending) != 1, "pending = %d, want 1", len(pending))
	require.Falsef(t, pending[0].State != OperationRecovering, "state = %q, want recovering", pending[0].State)
	require.Falsef(t, pending[0].StartedAt != updatedAt, "started_at = %s, want %s", pending[0].StartedAt, updatedAt)
	require.Falsef(t, pending[0].ID != "recovered-node-startup-workloads-workload-workloads-1774001100000000000", "id = %q, want deterministic recovered id", pending[0].ID)
	require.Truef(t, strings.Contains(pending[0].Reason, "invalid persisted operation state"), "reason = %q, want invalid persisted operation state", pending[0].Reason)
}

func TestRecorderKeepsFailedPrimaryAsFailedHealth(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "operations.json"))
	rec.SetPrimary(HealthFailed, &Reason{
		Code:                   "node.transport.start_failed",
		Domain:                 "transport",
		Summary:                "transport start failed",
		Recovery:               "restart_required",
		OperatorActionRequired: true,
	})

	snapshot := rec.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthFailed, "health = %q, want failed", snapshot.Health.State)
}

func TestRecorderKeepsFailedSubsystemAsFailedHealth(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "operations.json"))
	rec.SetSubsystem("transport", HealthFailed, &Reason{
		Code:                   "node.transport.start_failed",
		Domain:                 "transport",
		Summary:                "transport start failed",
		Recovery:               "restart_required",
		OperatorActionRequired: true,
	})

	snapshot := rec.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthFailed, "health = %q, want failed", snapshot.Health.State)
}

func TestRecorderSnapshotRedactsSensitivePayloadFields(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "operations.json"))
	rec.RecordEvent("data", "blob_fetched", "blob-1", "data.blob_fetched", "", map[string]any{
		"payload":      "plaintext-by-mistake",
		"secret":       "top-secret",
		"nested":       map[string]any{"token": "abc"},
		"safe_field":   "ok",
		"public_id":    "blob-1",
		"key_material": "123",
		"ciphertext":   "encrypted-bytes",
		"nonce":        "nonce-value",
		"private_key":  "pem",
		"seed":         "seed-value",
	})

	snapshot := rec.Snapshot()
	require.Falsef(t, len(snapshot.RecentEvents) != 1, "events = %d, want 1", len(snapshot.RecentEvents))

	payload := snapshot.RecentEvents[0].Payload
	require.Falsef(t, payload["payload"] != "[redacted]", "payload field = %#v, want redacted", payload["payload"])
	require.Falsef(t, payload["secret"] != "[redacted]", "secret field = %#v, want redacted", payload["secret"])
	require.Falsef(t, payload["key_material"] != "[redacted]", "key_material field = %#v, want redacted", payload["key_material"])
	require.Falsef(t, payload["ciphertext"] != "[redacted]", "ciphertext field = %#v, want redacted", payload["ciphertext"])
	require.Falsef(t, payload["nonce"] != "[redacted]", "nonce field = %#v, want redacted", payload["nonce"])
	require.Falsef(t, payload["private_key"] != "[redacted]", "private_key field = %#v, want redacted", payload["private_key"])
	require.Falsef(t, payload["seed"] != "[redacted]", "seed field = %#v, want redacted", payload["seed"])

	nested, ok := payload["nested"].(map[string]any)
	require.Truef(t, ok, "nested payload type = %T, want map[string]any", payload["nested"])
	require.Falsef(t, nested["token"] != "[redacted]", "nested token = %#v, want redacted", nested["token"])
	require.Falsef(t, payload["safe_field"] != "ok", "safe field = %#v, want ok", payload["safe_field"])
}

func TestRecorderSnapshotRedactsSensitiveValuesInsideArrays(t *testing.T) {
	rec := New(filepath.Join(t.TempDir(), "operations.json"))
	rec.RecordEvent("data", "blob_fetched", "blob-1", "data.blob_fetched", "", map[string]any{
		"items": []any{
			map[string]any{"token": "abc"},
			map[string]any{"safe": "ok", "nested": []any{map[string]any{"plaintext": "secret"}}},
		},
	})

	snapshot := rec.Snapshot()
	require.Falsef(t, len(snapshot.RecentEvents) != 1, "events = %d, want 1", len(snapshot.RecentEvents))

	items, ok := snapshot.RecentEvents[0].Payload["items"].([]any)
	require.Falsef(t, !ok || len(items) != 2, "items payload = %#v, want two array entries", snapshot.RecentEvents[0].Payload["items"])

	first, ok := items[0].(map[string]any)
	require.Falsef(t, !ok || first["token"] != "[redacted]", "first array item = %#v, want token redacted", items[0])

	second, ok := items[1].(map[string]any)
	require.Falsef(t, !ok || second["safe"] != "ok", "second array item = %#v, want safe field preserved", items[1])

	nested, ok := second["nested"].([]any)
	require.Falsef(t, !ok || len(nested) != 1, "nested array = %#v, want one entry", second["nested"])

	inner, ok := nested[0].(map[string]any)
	require.Falsef(t, !ok || inner["plaintext"] != "[redacted]", "nested array item = %#v, want plaintext redacted", nested[0])
}

func TestRecorderSurfacesPersistenceFailureUntilSaveRecovers(t *testing.T) {
	dir := t.TempDir()
	rec := New(dir)

	rec.RecordEvent("node", "starting", "node", "node startup started", "", nil)
	snapshot := rec.Snapshot()
	require.Falsef(t, snapshot.Health.State != HealthDegraded, "health = %q, want degraded", snapshot.Health.State)
	require.Falsef(t, snapshot.Health.PrimaryReason == nil || snapshot.Health.PrimaryReason.Code != persistenceFailureCode, "primary reason = %#v, want %s", snapshot.Health.PrimaryReason, persistenceFailureCode)

	foundSubsystem := false
	for _, item := range snapshot.Health.Subsystems {
		if item.Domain != "diagnostics" {
			continue
		}
		foundSubsystem = true
		require.Falsef(t, item.Reason == nil || item.Reason.Code != persistenceFailureCode, "subsystem reason = %#v, want %s", item.Reason, persistenceFailureCode)
	}
	require.True(t, foundSubsystem, "expected diagnostics persistence subsystem")

	rec.SetPath(filepath.Join(dir, "operations.json"))
	rec.RecordEvent("node", "started", "node", "node startup completed", "", nil)
	recovered := rec.Snapshot()
	require.Falsef(t, recovered.Health.State != HealthReady, "health = %q, want ready after persistence recovery", recovered.Health.State)

	for _, item := range recovered.Health.Subsystems {
		require.Falsef(t, item.Domain == "diagnostics", "unexpected diagnostics subsystem after recovery: %#v", item)
	}
}

func TestRecorderMinimalDetailOmitsResourceAndPayload(t *testing.T) {
	recorder := NewInDir(t.TempDir())
	recorder.SetDetailLevel("minimal")
	recorder.RecordEvent("network", "failure", "peer-1", "dial failed", "network.dial", map[string]any{"route": "private"})
	events := recorder.Snapshot().RecentEvents
	require.Len(t, events, 1)
	require.Empty(t, events[0].Resource)
	require.Nil(t, events[0].Payload)
	require.Equal(t, "network.dial", events[0].ReasonCode)
}
