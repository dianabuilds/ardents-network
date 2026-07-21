//go:build integration

package content_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	networkprivacy "ardents/internal/messaging"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDataSubstrateObjectAndBlobPersistAcrossRestart(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)

	dir := t.TempDir()
	first := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-persist",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	object, err := first.PublishObject(appdata.Object{
		Type:  "chat.message",
		Owner: "node",
		Body:  map[string]any{"text": "hello"},
	})
	require.NoErrorf(t, err, "publish object: %v", err)

	blob, err := first.PublishBlob(appdata.PublishBlobCommand{Blob: appdata.Blob{
		MediaType: "text/plain", Size: 12, Hash: "sha256:blob",
	}})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	second := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-persist",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	storedObject, ok := testkit.Content(second).GetObject(object.ID)
	require.True(t, ok, "get object")
	require.Falsef(t, storedObject.Body["text"] != "hello", "text = %v, want hello", storedObject.Body["text"])

	storedBlob, ok := testkit.Content(second).GetBlob(blob.ID)
	require.True(t, ok, "get blob")
	require.Falsef(t, storedBlob.MediaType !=
		"text/plain" || storedBlob.
		Size !=
		12, "blob = %#v, want text/plain size 12", storedBlob)

}

func TestDataSubstrateRestartReconcilesMissingPinnedPayloadBlob(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)

	dir := t.TempDir()
	first := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-missing-payload",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	blob, err := first.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "text/plain"}, Payload: []byte("pinned blob"),
	})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		_, err := first.PinBlob(blob.ID)
		require.NoErrorf(t, err, "pin blob: %v", err)
	}

	payloadPath := filepath.Join(dir, "blobs", strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(blob.ID)+".blob")
	{
		err := os.Remove(payloadPath)
		require.NoErrorf(t, err, "remove payload: %v", err)
	}
	{

		err := first.Stop(context.Background())
		require.NoErrorf(t, err, "stop first node: %v", err)
	}

	second := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-missing-payload",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: dir},
	})

	item, ok := testkit.Content(second).GetBlob(blob.ID)
	require.True(t, ok, "get blob")
	require.Falsef(t, item.State != "deleted", "state = %q, want deleted", item.State)

	inv := testkit.Content(second).InventorySnapshot()
	require.Falsef(t, inv.LocalBlobs != 0 ||
		inv.
			AvailableForResend != 0 ||
		inv.Deleted !=
			1, "inventory = %#v, want deleted-only truth", inv)

}

func TestDataSubstrateOperationsEmitDiagnosticsEvents(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)

	n := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-diag-events",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
	})

	blob, err := n.PublishBlob(appdata.PublishBlobCommand{Blob: appdata.Blob{MediaType: "text/plain"}})
	require.NoErrorf(t, err, "publish blob: %v", err)
	{

		_, err := n.PublishManifest(appdata.Manifest{
			Kind:  "message-attachment",
			Owner: "node",
			Refs:  []appdata.Ref{{Kind: "blob", ID: blob.ID}},
		})
		require.NoErrorf(t, err, "publish manifest: %v", err)
	}

	snapshot := testkit.Diagnostics(n).DiagnosticsSnapshot()
	foundBlob := false
	foundManifest := false
	for _, evt := range snapshot.RecentEvents {
		if evt.Domain != "data" {
			continue
		}
		if evt.Type == "blob_published" {
			foundBlob = true
		}
		if evt.Type == "manifest_published" {
			foundManifest = true
		}
	}
	require.Falsef(t, !foundBlob || !foundManifest, "data events found: blob=%v manifest=%v", foundBlob, foundManifest)

}

func TestDataSubstrateRejectsPlaintextRemoteReserve(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	{
		err := sourceStore.Load()
		require.NoErrorf(t, err, "load source store: %v", err)
	}

	stored, err := sourceStore.StoreBlob(appdata.Blob{MediaType: "text/plain"}, []byte("network payload"))
	require.NoErrorf(t, err, "store source blob: %v", err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "data-source-plaintext",
		Boot:    runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data:    runtimeinfra.DataConfig{Dir: sourceDir},
		Privacy: privacy.Sender,
	})

	records, err := source.ListRecords()
	require.NoErrorf(t, err, "list source records: %v", err)
	require.False(t, len(records) == 0, "expected source record")

	requesterDir := t.TempDir()
	requester := testkit.StartNode(t, runtimeinfra.Config{
		Name:    "data-requester-plaintext",
		Boot:    runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Endpoints...)},
		Trust:   runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
		Data:    runtimeinfra.DataConfig{Dir: requesterDir},
		Privacy: privacy.Receiver,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	{
		_, err := requester.FetchBlob(ctx, stored.ID)
		require.Falsef(t, err == nil || !strings.
			Contains(err.Error(), "plaintext blob re-serve is not allowed"), "error = %v, want terminal plaintext reserve rejection", err)
	}

	requesterStore := appdata.NewInDir(requesterDir)
	{
		err := requesterStore.Load()
		require.NoErrorf(t, err, "load requester store: %v", err)
	}
	{

		_, ok := requesterStore.GetBlob(stored.ID)
		require.False(t, ok, "expected requester to keep plaintext blob unavailable locally")
	}

}

func TestDataSubstrateBlobResponseRequiresDiscoveredRequester(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "data-substrate",
		ScenarioID:  "DAI-001",
		Suite:       "integration",
		Tags:        []string{"integration", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})
	testkit.ConfigureLoopbackTransport(t)

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	{
		err := sourceStore.Load()
		require.NoErrorf(t, err, "load source store: %v", err)
	}

	key := []byte("fedcba9876543210fedcba9876543210")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoErrorf(t, err, "store encrypted source blob: %v", err)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "data-source-requester-authz",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	})

	records, err := source.ListRecords()
	require.NoErrorf(t, err, "list source records: %v", err)
	require.False(t, len(records) == 0, "expected source record")

	attacker := testkit.NewTransport()
	attacker.SetBootstrapNodes(append([]string(nil), records[0].Endpoints...))
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	{
		err := attacker.Start(ctx)
		require.NoErrorf(t, err, "start attacker transport: %v", err)
	}

	defer func() { _ = attacker.Stop(context.Background()) }()

	requester := "undiscovered-requester"
	requestID := "attack-request"
	privacy := testkit.NewDataPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	exchange := testkit.NewPrivateExchange(privacy.Receiver, attacker)
	require.NoError(t, exchange.Start(ctx))
	responses, unregister, err := exchange.RegisterResponse(requestID)
	require.NoErrorf(t, err, "subscribe response topic: %v", err)
	defer unregister()

	wire, err := json.Marshal(map[string]string{
		"request_id": requestID,
		"blob_id":    stored.ID,
		"requester":  requester,
	})
	require.NoErrorf(t, err, "marshal blob request: %v", err)
	{

		err := exchange.Publish(ctx, networkprivacy.MessageClassBlobFetchRequest, wire)
		require.NoErrorf(t, err, "publish blob request: %v", err)
	}

	select {
	case payload, ok := <-responses:
		require.True(t, ok, "expected signed terminal denial response")

		var response map[string]any
		{
			err := json.Unmarshal(payload, &response)
			require.NoErrorf(t, err, "decode blob response: %v", err)
		}
		require.Falsef(t, response["status"] != "error", "status = %v, want error", response["status"])
		require.Truef(t, strings.Contains(response["error"].(string), "blob requester identity is incomplete"), "error = %v, want requester identity denial", response["error"])

	case <-time.After(2 * time.Second):
		require.FailNow(t, "expected signed terminal denial response")
	}
}
