//go:build integration

package transfer_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDataSubstrateFetchesAndResumesChunkedPayloadOverPrivateWaku(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "data-substrate", ScenarioID: "DAI-003",
		Suite: "integration", Tags: []string{"integration", "data-substrate", "chunked-transfer"},
		Speed: "slow", Environment: "local",
	})
	privacy := testkit.NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second))
	sourceDir := t.TempDir()
	sourceStore := appdata.NewInDir(sourceDir)
	require.NoError(t, sourceStore.Load())
	key := bytes.Repeat([]byte{0x71}, 32)
	plaintext := bytes.Repeat([]byte("chunked-network-payload"), 10000)
	stored, err := sourceStore.StoreChunkedPayload(context.Background(), appdata.ChunkedPayloadSpec{
		Owner: "owner", MediaType: "application/octet-stream", KeyID: "network-key-1",
	}, bytes.NewReader(plaintext), key)
	require.NoError(t, err)
	require.Greater(t, stored.ChunkCount, 1)

	source := testkit.StartNode(t, runtimeinfra.Config{
		Name: "chunked-source", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: sourceDir}, Privacy: privacy.Sender,
	})
	records, err := source.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, records)

	requesterDir := t.TempDir()
	requester := testkit.StartNode(t, runtimeinfra.Config{
		Name: "chunked-requester", Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), records[0].Endpoints...)},
		Trust: runtimeinfra.TrustConfig{Anchors: []string{source.Snapshot().Ident.PublicKey}},
		Data:  runtimeinfra.DataConfig{Dir: requesterDir}, Privacy: privacy.Receiver,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := requester.FetchChunked(ctx, stored.Root.ID)
	require.NoError(t, err)
	require.Equal(t, stored.ChunkCount, result.ChunkCount)
	require.Equal(t, stored.ChunkCount, result.FetchedCount)
	require.Zero(t, result.ResumedCount)

	requesterStore := appdata.NewInDir(requesterDir)
	require.NoError(t, requesterStore.Load())
	reconstructed := make([]byte, 0, len(plaintext))
	for _, ref := range result.Root.Refs {
		chunk, decryptErr := requesterStore.DecryptBlobPayload(ref.ID, key)
		require.NoError(t, decryptErr)
		reconstructed = append(reconstructed, chunk...)
	}
	require.Equal(t, plaintext, reconstructed)

	resumed, err := requester.FetchChunked(ctx, stored.Root.ID)
	require.NoError(t, err)
	require.Zero(t, resumed.FetchedCount)
	require.Equal(t, stored.ChunkCount, resumed.ResumedCount)
}
