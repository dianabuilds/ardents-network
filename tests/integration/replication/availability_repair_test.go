//go:build integration

package replication_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeprocess "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	discoveryapi "ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
	appreplication "ardents/internal/replication"
	"ardents/internal/replication/availability"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDataAvailabilityRepairsCorruptReplicaToDifferentWakuPeer(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "data-substrate", ScenarioID: "DAI-004",
		Suite: "integration", Tags: []string{"integration", "data-substrate", "waku", "repair"},
		Speed: "default", Environment: "local",
	})
	now := time.Now().UTC().Truncate(time.Second)
	discoveryPrivacy := testkit.NewDiscoveryPrivacyGroupFixture(t, now, 3)
	dataPrivacy := testkit.NewDataPrivacyGroupFixture(t, now, 3)
	targetOneDir, targetTwoDir, sourceDir := t.TempDir(), t.TempDir(), t.TempDir()

	targetOneSeed := testkit.StartNode(t, replicaNodeConfig("repair-target-one", targetOneDir, discoveryPrivacy.Channels[0], dataPrivacy.Channels[0], []string{"local://bootstrap"}, nil))
	targetOneIdentity := targetOneSeed.Snapshot().Ident
	require.NoError(t, targetOneSeed.Stop(context.Background()))
	targetTwoSeed := testkit.StartNode(t, replicaNodeConfig("repair-target-two", targetTwoDir, discoveryPrivacy.Channels[1], dataPrivacy.Channels[1], []string{"local://bootstrap"}, nil))
	targetTwoIdentity := targetTwoSeed.Snapshot().Ident
	require.NoError(t, targetTwoSeed.Stop(context.Background()))
	sourceSeed := testkit.StartNode(t, replicaNodeConfig("repair-source", sourceDir, discoveryPrivacy.Channels[2], dataPrivacy.Channels[2], []string{"local://bootstrap"}, nil))
	sourceIdentity := sourceSeed.Snapshot().Ident
	require.NoError(t, sourceSeed.Stop(context.Background()))

	targetOne := testkit.StartNode(t, replicaNodeConfig("repair-target-one", targetOneDir, discoveryPrivacy.Channels[0], dataPrivacy.Channels[0], []string{"local://bootstrap"}, []string{sourceIdentity.PublicKey}))
	targetTwo := testkit.StartNode(t, replicaNodeConfig("repair-target-two", targetTwoDir, discoveryPrivacy.Channels[1], dataPrivacy.Channels[1], testkit.BootstrapEndpoints(t, targetOne), []string{sourceIdentity.PublicKey}))
	sourceBootstrap := append(testkit.BootstrapEndpoints(t, targetOne), testkit.BootstrapEndpoints(t, targetTwo)...)
	sourceConfig := replicaNodeConfig("repair-source", sourceDir, discoveryPrivacy.Channels[2], dataPrivacy.Channels[2], sourceBootstrap, []string{targetOneIdentity.PublicKey, targetTwoIdentity.PublicKey})
	source := testkit.StartNode(t, sourceConfig)
	importRepairNodeRecords(t, source, targetOne, targetTwo)

	blob, err := source.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "application/octet-stream",
			Encrypted: true, Cipher: appdata.BlobCipherAES256GCM, KeyID: "key-redacted"},
		Payload: []byte("repairable ciphertext"),
	})
	require.NoError(t, err)
	sourcePrincipal, err := identityprincipal.Parse(sourceIdentity.Principal)
	require.NoError(t, err)
	root, err := source.PublishManifest(appdata.Manifest{
		Kind: "blob-set", Owner: sourcePrincipal, Encrypted: true, Retention: "durable",
		Refs: []appdata.Ref{{Kind: "blob", ID: blob.ID}},
	})
	require.NoError(t, err)
	sourceNode := source
	_, err = source.SetReplicaIntent(availability.ReplicaIntent{
		ID: "repair-intent", RootManifestID: root.ID, Version: 1, DesiredCopies: 2, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, Retention: "durable",
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	commitment, err := runtimeprocess.PlaceBlobReplicaForIntegrationTest(sourceNode, ctx, blob.ID, targetOneIdentity.Principal, 1)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(replicaPayloadPath(targetOneDir, blob.ID), []byte("corrupt protected payload"), 0o600))

	corrupt, err := runtimeprocess.ProbeBlobReplicaForIntegrationTest(sourceNode, ctx, commitment)
	require.Error(t, err)
	require.Equal(t, appreplication.ReplicaCommitmentCorrupt, corrupt.State)
	require.NoError(t, source.ReconcileDataAvailability(ctx))

	snapshot, err := source.GetAvailability(root.ID)
	require.NoError(t, err)
	require.Equal(t, "target-satisfied", snapshot.State)
	require.Equal(t, 2, snapshot.ValidCopies)
	require.Equal(t, 1, snapshot.CorruptCopies)
	state := runtimeprocess.ReplicaPlacementStateForIntegrationTest(sourceNode)
	require.True(t, hasActiveCommitmentForPeer(state, targetTwoIdentity.Principal))
	events, _ := testkit.Diagnostics(source).ListRecentEvents(100, "")
	require.True(t, hasDataEvent(events, "replica_repaired"))
	require.True(t, hasDataEvent(events, "availability_observed"))

	updatedAt := time.Now().UTC()
	_, err = source.SetReplicaIntent(availability.ReplicaIntent{
		ID: "repair-intent", RootManifestID: root.ID, Version: 2, DesiredCopies: 2, MinimumCopies: 1,
		LeaseDuration: 24 * time.Hour, RenewalHorizon: 8 * time.Hour, Retention: "durable",
		CreatedAt: now, UpdatedAt: updatedAt,
	})
	require.NoError(t, err)
	peerLossCommitment, err := runtimeprocess.PlaceBlobReplicaForIntegrationTest(sourceNode, ctx, blob.ID, targetOneIdentity.Principal, 2)
	require.NoError(t, err)
	require.NoError(t, runtimeprocess.StopTransportForIntegrationTest(targetOne, context.Background()))
	probeCtx, probeCancel := context.WithTimeout(t.Context(), 4*time.Second)
	defer probeCancel()
	_, err = runtimeprocess.ProbeBlobReplicaForIntegrationTest(sourceNode, probeCtx, peerLossCommitment)
	require.Error(t, err)
	lossCtx, lossCancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer lossCancel()
	require.NoError(t, source.ReconcileDataAvailability(lossCtx))
	snapshot, err = source.GetAvailability(root.ID)
	require.NoError(t, err)
	require.Equal(t, uint64(2), snapshot.IntentVersion)
	require.Equal(t, "target-satisfied", snapshot.State)
	require.Equal(t, 2, snapshot.ValidCopies)
	require.GreaterOrEqual(t, snapshot.StaleCopies, 1)

	require.NoError(t, source.Stop(context.Background()))
	retainedWhileOwnerOffline, ok := testkit.Content(targetTwo).GetBlob(blob.ID)
	require.True(t, ok)
	require.True(t, retainedWhileOwnerOffline.Encrypted)
	restarted := testkit.StartNode(t, sourceConfig)
	persisted, err := restarted.GetAvailability(root.ID)
	require.NoError(t, err)
	require.Equal(t, snapshot.IntentVersion, persisted.IntentVersion)
	require.Equal(t, snapshot.State, persisted.State)
	require.Equal(t, snapshot.ValidCopies, persisted.ValidCopies)
	require.Equal(t, snapshot.StaleCopies, persisted.StaleCopies)
}

func importRepairNodeRecords(t *testing.T, source, targetOne, targetTwo *runtimeprocess.Node) {
	t.Helper()
	filter := func(record discoveryapi.CatalogRecordSnapshot) bool { return record.Kind() == "node" }
	testkit.ImportRecordsFromNode(t, source, targetOne, "repair-target-one", filter)
	testkit.ImportRecordsFromNode(t, source, targetTwo, "repair-target-two", filter)
	testkit.ImportRecordsFromNode(t, targetOne, source, "repair-source", filter)
	testkit.ImportRecordsFromNode(t, targetTwo, source, "repair-source", filter)
}

func replicaPayloadPath(dir, id string) string {
	safeID := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(id)
	return filepath.Join(dir, "blobs", safeID+".blob")
}

func hasActiveCommitmentForPeer(state appreplication.ReplicaPlacementSnapshot, peer string) bool {
	for _, commitment := range state.Commitments {
		if commitment.TargetNode.String() == peer && commitment.State == appreplication.ReplicaCommitmentActive {
			return true
		}
	}
	return false
}

func hasDataEvent(events []diagapi.EventEnvelope, eventType string) bool {
	for _, event := range events {
		if event.Domain == "data" && event.Type == eventType {
			return true
		}
	}
	return false
}
