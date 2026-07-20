//go:build e2e

package datasubstratee2e_test

import (
	"context"
	"errors"
	"testing"
	"time"

	appdata "ardents/internal/data"
	discoveryapi "ardents/internal/discovery/api"
	nodeapi "ardents/internal/node/api"
	runtimeinfra "ardents/internal/runtime/process"
	runtimeprocess "ardents/internal/runtime/process"
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
		Data: runtimeinfra.NodeDataConfig{Dir: sourceDir},
	}

	var source dataSourceRuntime
	var trusted runtimeprocess.NodeRuntime
	var trustedDir string
	var untrusted runtimeprocess.NodeRuntime

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
			Boot:  runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Endpoints...)},
			Trust: runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
			Data:  runtimeinfra.NodeDataConfig{Dir: trustedDir},
		}).Runtime
		require.NoError(t, trusted.Start(context.Background()))
		t.Cleanup(func() {
			_ = trusted.Stop(context.Background())
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		blob, err := trusted.FetchBlob(ctx, stored.ID)
		require.NoError(t, err)
		require.Equal(t, stored.ID, blob.ID)
		require.True(t, blob.Encrypted)
	})

	scenario.Assert("trusted requester keeps encrypted local truth and inventory", func(t *testing.T) {
		blob, err := trusted.GetBlob(stored.ID)
		require.NoError(t, err)
		require.True(t, blob.Encrypted)

		inventory := trusted.DataInventory()
		require.Equal(t, 1, inventory.LocalBlobs)
		require.Equal(t, 1, inventory.Encrypted)

		requesterStore := appdata.NewInDir(trustedDir)
		require.NoError(t, requesterStore.Load())
		plaintext, err := requesterStore.DecryptBlobPayload(stored.ID, key)
		require.NoError(t, err)
		require.Equal(t, "network payload", string(plaintext))
	})

	scenario.Degraded("untrusted requester gets explicit failure without false local availability", func(t *testing.T) {
		records, err := source.ListRecords()
		require.NoError(t, err)
		require.NotEmpty(t, records)

		untrusted = testkit.NewRuntime(t, runtimeinfra.Config{
			Name: "data-e2e-untrusted",
			Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Endpoints...)},
			Data: runtimeinfra.NodeDataConfig{Dir: t.TempDir()},
		}).Runtime
		require.NoError(t, untrusted.Start(context.Background()))
		t.Cleanup(func() {
			_ = untrusted.Stop(context.Background())
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err = untrusted.FetchBlob(ctx, stored.ID)
		require.Error(t, err)
		require.False(t, errors.Is(err, context.DeadlineExceeded))
		require.NotContains(t, err.Error(), context.DeadlineExceeded.Error())

		inventory := untrusted.DataInventory()
		require.Equal(t, 0, inventory.LocalBlobs)
		require.Equal(t, 0, inventory.Encrypted)

		_, getErr := untrusted.GetBlob(stored.ID)
		require.Error(t, getErr)
		require.Contains(t, getErr.Error(), "not found")
	})
}

type dataSourceRuntime interface {
	runtimeprocess.NodeRuntime
	ListRecords() ([]discoveryapi.DiscoveryRecord, error)
	Snapshot() nodeapi.Snapshot
}
