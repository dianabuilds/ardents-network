//go:build e2e

package content_e2e_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDataSubstrateRemoteFetchAndUnavailableTruth(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerE2E,
		Domain:      "data-substrate",
		ScenarioID:  "DAE-001",
		Suite:       "e2e",
		Tags:        []string{"integration", "e2e", "data-substrate"},
		Speed:       "default",
		Environment: "local",
	})

	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := []byte("0123456789abcdef0123456789abcdef")
	stored, err := sourceStore.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream"}, []byte("network payload"), key, "")
	require.NoError(t, err)

	sourceCfg := runtimeinfra.Config{
		Name: "data-e2e-source",
		Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir},
	}

	var source *runtimeprocess.Node
	var trusted *runtimeprocess.Node
	var trustedDir string
	var untrusted *runtimeprocess.Node

	scenario.Precondition("start source node with encrypted remote blob", func(t *testing.T) {
		source = testkit.NewRuntime(t, sourceCfg).Node
		require.NoError(t, source.Start(context.Background()))
		t.Cleanup(func() {
			_ = source.Stop(context.Background())
		})
	})

	scenario.Step("trusted requester fetches remote encrypted blob via local data surface", func(t *testing.T) {
		records, err := source.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)

		trustedDir = t.TempDir()
		trusted = testkit.NewRuntime(t, runtimeinfra.Config{
			Name:  "data-e2e-trusted",
			Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].EndpointList()...)},
			Trust: runtimeinfra.TrustConfig{Registry: testkit.DiscoveryTrustRegistry(t, source.Snapshot().Ident.PublicKey)},
			Data:  runtimeinfra.DataConfig{Dir: trustedDir},
		}).Runtime
		require.NoError(t, trusted.Start(context.Background()))
		t.Cleanup(func() {
			_ = trusted.Stop(context.Background())
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		blob, err := trusted.FetchBlob(ctx, stored.Reference.String())
		require.NoError(t, err)
		require.Equal(t, stored.Reference.String(), blob.Reference.String())
		require.True(t, blob.Encrypted)
	})

	scenario.Assert("trusted requester keeps encrypted local truth and inventory", func(t *testing.T) {
		blob, ok := testkit.Content(trusted).GetBlob(stored.Reference.String())
		require.True(t, ok)
		require.True(t, blob.Encrypted)

		inventory := testkit.Content(trusted).InventorySnapshot()
		require.Equal(t, 1, inventory.LocalBlobs)
		require.Equal(t, 1, inventory.Encrypted)

		requesterStore := appdata.NewInDir(trustedDir)
		require.NoError(t, requesterStore.Load())
		plaintext, err := requesterStore.DecryptBlobPayload(stored.Reference.String(), key)
		require.NoError(t, err)
		require.Equal(t, "network payload", string(plaintext))
	})

	scenario.Degraded("untrusted requester gets explicit failure without false local availability", func(t *testing.T) {
		records, err := source.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)

		untrusted = testkit.NewRuntime(t, runtimeinfra.Config{
			Name: "data-e2e-untrusted",
			Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].EndpointList()...)},
			Data: runtimeinfra.DataConfig{Dir: t.TempDir()},
		}).Runtime
		require.NoError(t, untrusted.Start(context.Background()))
		t.Cleanup(func() {
			_ = untrusted.Stop(context.Background())
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err = untrusted.FetchBlob(ctx, stored.Reference.String())
		require.Error(t, err)
		require.False(t, errors.Is(err, context.DeadlineExceeded))
		require.NotContains(t, err.Error(), context.DeadlineExceeded.Error())

		inventory := testkit.Content(untrusted).InventorySnapshot()
		require.Equal(t, 0, inventory.LocalBlobs)
		require.Equal(t, 0, inventory.Encrypted)

		_, ok := testkit.Content(untrusted).GetBlob(stored.Reference.String())
		require.False(t, ok)
	})
}
