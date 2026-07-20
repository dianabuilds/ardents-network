package recorder

import (
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	want := Ledger{
		Seq: 7,
		Health: health.Summary{
			State:         health.Degraded,
			UpdatedAt:     time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC),
			PrimaryReason: &reason.Reason{Code: "boot.degraded", Domain: "boot", Summary: "boot degraded"},
		},
	}
	{
		err := Save(path, want)
		require.NoErrorf(t, err, "save: %v", err)
	}

	got, err := Load(path)
	require.NoErrorf(t, err, "load: %v", err)
	require.Falsef(t, got.Seq != want.Seq, "seq = %d, want %d", got.Seq, want.Seq)
	require.Falsef(t, got.Health.PrimaryReason == nil || got.Health.PrimaryReason.Code != "boot.degraded", "primary reason = %#v, want boot.degraded", got.Health.PrimaryReason)
}

func TestDecodeReturnsFatalCorruptLedger(t *testing.T) {
	_, err := Decode([]byte("{broken"))
	require.Error(t, err, "expected error")

	corrupt, ok := IsCorruptLedger(err)
	require.Falsef(t, !ok || !corrupt.Fatal, "err = %v, want fatal corrupt ledger", err)
}

func TestLoadTreatsMissingFileAsEmptyLedger(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	require.NoErrorf(t, err, "load: %v", err)
	require.Falsef(t, got.Seq != 0, "seq = %d, want 0", got.Seq)
}

func TestLoadTreatsEmptyFileAsEmptyLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	{
		err := os.WriteFile(path, []byte(" \n"), 0o644)
		require.NoErrorf(t, err, "write: %v", err)
	}

	got, err := Load(path)
	require.NoErrorf(t, err, "load: %v", err)
	require.Falsef(t, got.Seq != 0, "seq = %d, want 0", got.Seq)
}

func TestSaveCompactsClosedOperationsButKeepsOpenOnes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.json")
	start := time.Date(2026, 3, 22, 18, 0, 0, 0, time.UTC)
	items := make([]operation.Record, 0, maxClosedOperations+10)
	for i := 0; i < maxClosedOperations+5; i++ {
		started := start.Add(time.Duration(i) * time.Minute)
		finished := started.Add(30 * time.Second)
		items = append(items, operation.Record{
			ID:         "closed-" + started.Format("150405"),
			Kind:       "node.shutdown",
			State:      operation.Completed,
			Domain:     "node",
			Resource:   "node",
			StartedAt:  started,
			UpdatedAt:  finished,
			FinishedAt: &finished,
		})
	}
	openStarted := start.Add(24 * time.Hour)
	items = append(items, operation.Record{
		ID:          "open-running",
		Kind:        "node.startup.workloads",
		State:       operation.Running,
		Domain:      "workload",
		Resource:    "workloads",
		Recoverable: true,
		StartedAt:   openStarted,
		UpdatedAt:   openStarted,
	})
	{
		err := Save(path, Ledger{Operations: items})
		require.NoErrorf(t, err, "save: %v", err)
	}

	got, err := Load(path)
	require.NoErrorf(t, err, "load: %v", err)
	require.Falsef(t, len(got.Operations) != maxClosedOperations+1, "operations = %d, want %d", len(got.Operations), maxClosedOperations+1)
	require.Falsef(t, got.Operations[len(got.Operations)-1].ID != "open-running", "last operation = %q, want open-running", got.Operations[len(got.Operations)-1].ID)

	for i := 0; i < 5; i++ {
		id := "closed-" + start.Add(time.Duration(i)*time.Minute).Format("150405")
		for _, item := range got.Operations {
			require.Falsef(t, item.ID == id, "found compacted-out closed operation %q", id)
		}
	}
}
