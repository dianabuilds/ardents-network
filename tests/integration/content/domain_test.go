//go:build integration

package content_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	db "ardents/internal/storage"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

type persistedDataSnapshot struct {
	Objects       map[string]appdata.Object             `json:"objects"`
	Blobs         map[string]appdata.Blob               `json:"blobs"`
	Sources       map[string][]appdata.BlobSourceRecord `json:"sources"`
	Manifests     map[string]appdata.Manifest           `json:"manifests"`
	BlobOwnership json.RawMessage                       `json:"blob_ownership"`
}

func TestDataSubstrateRestartReconcilesExpiredRetention(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()

	first := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-restart",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	published, err := first.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "text/plain"}, Payload: []byte("expire on restart"),
	})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		_, err := first.RetainBlob(published.ID, time.Now().UTC().Add(-time.Minute))
		require.NoErrorf(t, err, "retain blob: %v", err)
	}
	{

		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	second := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-restart",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	blob, ok := testkit.Content(second).GetBlob(published.ID)
	require.True(t, ok, "get blob")
	require.Falsef(t, blob.State != "expired", "state = %q, want expired", blob.State)

	inv := testkit.Content(second).InventorySnapshot()
	require.Falsef(t, inv.RetainedTemporary !=
		0 || inv.LocalBlobs != 0 ||
		inv.AvailableForResend !=
			0, "inventory = %#v, want no retained local truth after restart reconcile", inv)
	require.Falsef(t, inv.Expired != 1, "expired = %d, want 1", inv.Expired)

}

func TestDataSubstrateFetchesEncryptedBlobFromTrustedPeer(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	{
		err := sourceStore.Load()
		require.NoErrorf(t, err, "load source data store: %v", err)
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoErrorf(t, err, "store encrypted source blob: %v", err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "data-source",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:    runtimeinfra.DataConfig{Dir: sourceDir},
		Privacy: privacy.Sender,
	})

	records, err := source.ListRecords()
	require.NoErrorf(t, err, "list source records: %v", err)
	require.False(t, len(records) == 0, "expected source discovery records")

	requesterDir := t.TempDir()
	requesterBoot := append([]string(nil), records[0].Endpoints...)
	requester := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "data-requester",
		Boot:    runtimeinfra.BootConfig{Sources: requesterBoot},
		Trust:   runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
		Data:    runtimeinfra.DataConfig{Dir: requesterDir},
		Privacy: privacy.Receiver,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	blob, err := requester.FetchBlob(ctx, stored.ID)
	require.NoErrorf(t, err, "fetch blob: %v", err)
	require.Falsef(t, blob.ID != stored.ID, "blob id = %q, want %q", blob.ID, stored.ID)
	require.True(t, blob.Encrypted, "expected fetched blob to stay encrypted")

	requesterStore := appdata.NewInDir(requesterDir)
	{
		err := requesterStore.Load()
		require.NoErrorf(t, err, "load requester data store: %v", err)
	}

	plaintext, err := requesterStore.DecryptBlobPayload(stored.ID, key)
	require.NoErrorf(t, err, "decrypt fetched payload: %v", err)
	require.Falsef(t, string(plaintext) != "network payload", "payload = %q, want network payload", string(plaintext))

}

func TestDataSubstrateRejectsFetchFromUntrustedPeer(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	{
		err := sourceStore.Load()
		require.NoErrorf(t, err, "load source data store: %v", err)
	}

	key := []byte("abcdef0123456789abcdef0123456789")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoErrorf(t, err, "store encrypted source blob: %v", err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-source-untrusted",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	})

	records, err := source.ListRecords()
	require.NoErrorf(t, err, "list source records: %v", err)
	require.False(t, len(records) == 0, "expected source discovery records")

	requesterDir := t.TempDir()
	requester := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-requester-untrusted",
		Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Endpoints...)},
		Data: runtimeinfra.DataConfig{Dir: requesterDir},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := requester.FetchBlob(ctx, stored.ID); err == nil {
		require.FailNow(t, "expected fetch failure for untrusted peer")
	} else if errors.Is(err, context.DeadlineExceeded) {
		require.FailNowf(t, "unexpected timeout", "error = %v, want explicit terminal denial instead of timeout", err)
	}

	requesterStore := appdata.NewInDir(requesterDir)
	{
		err := requesterStore.Load()
		require.NoErrorf(t, err, "load requester data store: %v", err)
	}
	{

		_, ok := requesterStore.GetBlob(stored.ID)
		require.False(t, ok, "expected requester to keep blob unavailable locally")
	}

}

func TestDataSubstrateSnapshotExplainsBlobPayloadLoss(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-blob-truth",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	blob, err := n.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "text/plain"}, Payload: []byte("payload disappears"),
	})
	require.NoErrorf(t, err, "publish blob: %v", err)

	payloadPath := filepath.Join(dir, "blobs", strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(blob.ID)+".blob")
	{
		err := os.Remove(payloadPath)
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	snapshot := n.Snapshot()
	require.Falsef(t, snapshot.Blob.State !=
		"degraded", "blob state = %q, want degraded", snapshot.Blob.State)
	require.Truef(t, strings.Contains(snapshot.
		Blob.Reason, blob.ID), "blob reason = %q, want blob-specific explanation", snapshot.Blob.Reason)
	require.Falsef(t, snapshot.Object.State !=
		"ready", "object state = %q, want ready", snapshot.Object.State)

}

func TestDataSubstrateSnapshotExplainsBrokenObjectRefsAfterRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	dir := t.TempDir()
	first := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-object-truth",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	blob, err := first.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "text/plain"}, Payload: []byte("object ref payload"),
	})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		_, err := first.PublishObject(appdata.Object{
			Type:  "chat.message",
			Owner: "principal.local",
			BlobRefs: []appdata.Ref{{
				Kind: "blob",
				ID:   blob.ID,
			}},
		})
		require.NoErrorf(t, err, "publish object: %v", err)
	}
	{

		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	dbPath := db.PathInDir(dir)
	var persisted persistedDataSnapshot
	{
		_, err := db.LoadJSON(dbPath, "data", "snapshot", &persisted)
		require.NoErrorf(t, err, "load persisted snapshot: %v", err)
	}

	delete(persisted.Blobs, blob.ID)
	{
		err := db.SaveJSON(dbPath, "data", "snapshot", persisted)
		require.NoErrorf(t, err, "save persisted snapshot: %v", err)
	}

	second := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-object-truth",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	snapshot := second.Snapshot()
	require.Falsef(t, snapshot.Object.State !=
		"degraded", "object state = %q, want degraded", snapshot.Object.State)
	require.Truef(t, strings.Contains(snapshot.
		Object.Reason, "missing blob",
	), "object reason = %q, want missing-blob explanation", snapshot.Object.Reason)
	require.Falsef(t, snapshot.Blob.State !=
		"ready", "blob state = %q, want ready", snapshot.Blob.State)

}
