//go:build integration

package replication_test

import (
	"context"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeprocess "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestDataReplicaPlacementCommitsEncryptedCopyOverPrivateWaku(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "data-substrate", ScenarioID: "DAI-002",
		Suite: "integration", Tags: []string{"integration", "data-substrate", "waku", "replication"},
		Speed: "default", Environment: "local",
	})
	now := time.Now().UTC().Truncate(time.Second)
	discoveryPrivacy := testkit.NewDiscoveryPrivacyFixture(t, now)
	dataPrivacy := testkit.NewDataPrivacyFixture(t, now)
	targetDir, sourceDir := t.TempDir(), t.TempDir()
	targetSeed := testkit.StartNode(t, replicaNodeConfig("replica-target", targetDir, discoveryPrivacy.Sender, dataPrivacy.Receiver, []string{"local://bootstrap"}, nil))
	targetIdentity := targetSeed.Snapshot().Ident
	targetPrincipal := targetIdentity.Principal
	require.NoError(t, targetSeed.Stop(context.Background()))
	sourceSeed := testkit.StartNode(t, replicaNodeConfig("replica-source", sourceDir, discoveryPrivacy.Receiver, dataPrivacy.Sender, []string{"local://bootstrap"}, nil))
	sourceIdentity := sourceSeed.Snapshot().Ident
	sourcePrincipal := sourceIdentity.Principal
	require.NoError(t, sourceSeed.Stop(context.Background()))

	target := testkit.StartNode(t, replicaNodeConfig("replica-target", targetDir, discoveryPrivacy.Sender, dataPrivacy.Receiver, []string{"local://bootstrap"}, []string{sourceIdentity.PublicKey}))
	source := testkit.StartNode(t, replicaNodeConfig("replica-source", sourceDir, discoveryPrivacy.Receiver, dataPrivacy.Sender, testkit.BootstrapEndpoints(t, target), []string{targetIdentity.PublicKey}))

	testkit.ImportRecordsFromNode(t, target, source, "replica-source", func(record discoveryapi.CatalogRecordSnapshot) bool { return record.Kind == "node" })
	require.Equal(t, sourcePrincipal, source.Snapshot().Ident.Principal)
	require.Equal(t, targetPrincipal, target.Snapshot().Ident.Principal)

	ciphertext := []byte("ciphertext-replica-payload")
	blob, err := source.PublishBlob(appdata.PublishBlobCommand{
		Blob: appdata.Blob{MediaType: "application/octet-stream",
			Encrypted: true, Cipher: appdata.BlobCipherAES256GCM, KeyID: "key-redacted"},
		Payload: ciphertext,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	sourceNode := source
	outcome, err := runtimeprocess.PlaceAvailableBlobReplicasForIntegrationTest(sourceNode, ctx, blob.ID, 1, 1)
	require.NoError(t, err)
	require.Len(t, outcome.Commitments, 1)
	require.Equal(t, []string{targetPrincipal}, outcome.Decision.SelectedNodeIDs())
	commitment := outcome.Commitments[0]
	require.Equal(t, targetPrincipal, commitment.PeerID)
	require.Equal(t, blob.CID, commitment.CID)
	require.True(t, commitment.LeaseExpiresAt.After(time.Now().UTC()))

	targetBlob, ok := testkit.Content(target).GetBlob(blob.ID)
	require.True(t, ok)
	require.True(t, targetBlob.Encrypted)
	require.Equal(t, "relay-temporary", targetBlob.Retention)

	targetState := runtimeprocess.ReplicaPlacementStateForIntegrationTest(target)
	sourceState := runtimeprocess.ReplicaPlacementStateForIntegrationTest(sourceNode)
	require.Equal(t, commitment, targetState.Commitments[commitment.OperationID])
	require.Equal(t, commitment, sourceState.Commitments[commitment.OperationID])

	renewed, err := runtimeprocess.ProbeBlobReplicaForIntegrationTest(sourceNode, ctx, commitment)
	require.NoError(t, err)
	require.True(t, renewed.LastObservedAt.After(commitment.LastObservedAt))
	require.True(t, renewed.LeaseExpiresAt.After(commitment.LeaseExpiresAt))
	targetState = runtimeprocess.ReplicaPlacementStateForIntegrationTest(target)
	sourceState = runtimeprocess.ReplicaPlacementStateForIntegrationTest(sourceNode)
	require.Equal(t, renewed, targetState.Commitments[commitment.OperationID])
	require.Equal(t, renewed, sourceState.Commitments[commitment.OperationID])
	targetBlob, ok = testkit.Content(target).GetBlob(blob.ID)
	require.True(t, ok)
	require.Equal(t, renewed.LeaseExpiresAt, targetBlob.ExpiresAt)
}

func replicaNodeConfig(name, dir string, privacy, dataPrivacy *networkprivacy.Channel, bootstrap, anchors []string) runtimeprocess.Config {
	return runtimeprocess.Config{
		Name: name, NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeprocess.BootConfig{Sources: bootstrap},
		Transport: runtimeprocess.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeprocess.DataConfig{Dir: dir, MaxRelayRetentionBytes: 1024 * 1024},
		Trust:     runtimeprocess.TrustConfig{Anchors: anchors},
		Privacy:   privacy, DataPrivacy: dataPrivacy, DiscoveryRefreshInterval: 50 * time.Millisecond,
	}
}
