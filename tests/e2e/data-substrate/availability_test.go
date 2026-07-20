//go:build e2e

package datasubstratee2e_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
	dataplacement "ardents/internal/data/placement"
	discoveryapi "ardents/internal/discovery/api"
	networkapi "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	runtimeprocess "ardents/internal/runtime/process"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestEncryptedAvailabilitySurvivesPeerLossRepairAndOwnerPayloadLoss(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "data-substrate", ScenarioID: "DAE-002",
		Suite: "e2e", Tags: []string{"integration", "e2e", "data-substrate", "waku", "repair"},
		Speed: "default", Environment: "local",
	})
	fixture := startAvailabilityFixture(t)
	root := publishAvailabilityManifest(t, fixture)
	setDurableIntent(t, fixture.owner, root.ID, fixture.now)

	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	require.NoError(t, fixture.owner.ReconcileDataAvailability(ctx))
	assertAvailability(t, fixture.owner, root.ID, "target-satisfied", 3)

	ownerNode := fixture.owner.NodeForTesting().(*runtimeprocess.Node)
	initial := activeRemoteCommitments(runtimeprocess.ReplicaPlacementStateForIntegrationTest(ownerNode))
	require.Len(t, initial, 2)
	lost := lossCommitment(initial, fixture.bootstrapPeer)
	require.NotEmpty(t, lost.OperationID)
	require.NoError(t, runtimeprocess.StopTransportForIntegrationTest(fixture.peers[lost.PeerID].NodeForTesting().(*runtimeprocess.Node), context.Background()))
	probeCtx, probeCancel := context.WithTimeout(t.Context(), 4*time.Second)
	_, probeErr := runtimeprocess.ProbeBlobReplicaForIntegrationTest(ownerNode, probeCtx, lost)
	probeCancel()
	require.Error(t, probeErr)
	require.NoError(t, fixture.owner.ReconcileDataAvailability(ctx))
	assertAvailability(t, fixture.owner, root.ID, "target-satisfied", 3)

	repaired := activeRemoteCommitments(runtimeprocess.ReplicaPlacementStateForIntegrationTest(ownerNode))
	require.Len(t, repaired, 2)
	require.NotContains(t, commitmentPeers(repaired), lost.PeerID)
	assertForeignCopiesAreCiphertext(t, fixture, repaired)

	_, err := fixture.owner.DropBlob(fixture.blob.ID)
	require.NoError(t, err)
	fetched, err := fixture.owner.FetchBlob(ctx, fixture.blob.ID)
	require.NoError(t, err)
	require.True(t, fetched.Encrypted)
	ownerStore := appdata.NewInDir(fixture.ownerDir)
	require.NoError(t, ownerStore.Load())
	plaintext, err := ownerStore.DecryptBlobPayload(fixture.blob.ID, fixture.key)
	require.NoError(t, err)
	require.Equal(t, fixture.plaintext, plaintext)
	require.NoError(t, fixture.owner.ReconcileDataAvailability(ctx))
	assertAvailability(t, fixture.owner, root.ID, "target-satisfied", 3)
	require.NotEmpty(t, fixture.owner.ListReplicaRepairs(root.ID))
	events, _ := fixture.owner.ListRecentEvents(100, "")
	require.True(t, containsDataEvent(events, "availability_observed"))
	require.True(t, containsDataEvent(events, "replica_repaired"))
}

type availabilityFixture struct {
	now           time.Time
	owner         runtimeprocess.NodeRuntime
	ownerDir      string
	blob          appdata.Blob
	key           []byte
	plaintext     []byte
	peers         map[string]runtimeprocess.NodeRuntime
	peerDirs      map[string]string
	bootstrapPeer string
}

func startAvailabilityFixture(t *testing.T) availabilityFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	discoveryPrivacy := testkit.NewDiscoveryPrivacyGroupFixture(t, now, 4)
	dataPrivacy := testkit.NewDataPrivacyGroupFixture(t, now, 4)
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	names := []string{"availability-peer-one", "availability-peer-two", "availability-peer-three", "availability-owner"}
	seeds := make([]runtimeprocess.NodeRuntime, 4)
	for index := range seeds {
		seeds[index] = testkit.StartNode(t, availabilityNodeConfig(names[index], dirs[index], discoveryPrivacy.Channels[index], dataPrivacy.Channels[index], []string{"local://bootstrap"}, nil))
	}
	identities := make([]string, 4)
	principals := make([]string, 4)
	for index, seed := range seeds {
		identities[index] = seed.Snapshot().Ident.PublicKey
		principals[index] = seed.Snapshot().Ident.Principal
		require.NoError(t, seed.Stop(context.Background()))
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	plaintext := []byte("owner-only cleartext for encrypted availability recovery")
	store := appdata.NewInDir(dirs[3])
	require.NoError(t, store.Load())
	blob, err := store.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream", Retention: "durable"}, plaintext, key, "owner-key-reference")
	require.NoError(t, err)

	peerOne := testkit.StartNode(t, availabilityNodeConfig(names[0], dirs[0], discoveryPrivacy.Channels[0], dataPrivacy.Channels[0], []string{"local://bootstrap"}, []string{identities[3]}))
	bootstrap := testkit.BootstrapEndpoints(t, peerOne)
	peerTwo := testkit.StartNode(t, availabilityNodeConfig(names[1], dirs[1], discoveryPrivacy.Channels[1], dataPrivacy.Channels[1], bootstrap, []string{identities[3]}))
	peerThreeBootstrap := append(append([]string(nil), bootstrap...), testkit.BootstrapEndpoints(t, peerTwo)...)
	peerThree := testkit.StartNode(t, availabilityNodeConfig(names[2], dirs[2], discoveryPrivacy.Channels[2], dataPrivacy.Channels[2], peerThreeBootstrap, []string{identities[3]}))
	owner := testkit.StartNode(t, availabilityNodeConfig(names[3], dirs[3], discoveryPrivacy.Channels[3], dataPrivacy.Channels[3], append(testkit.BootstrapEndpoints(t, peerOne), append(testkit.BootstrapEndpoints(t, peerTwo), testkit.BootstrapEndpoints(t, peerThree)...)...), identities[:3]))
	peers := map[string]runtimeprocess.NodeRuntime{principals[0]: peerOne, principals[1]: peerTwo, principals[2]: peerThree}
	peerDirs := map[string]string{principals[0]: dirs[0], principals[1]: dirs[1], principals[2]: dirs[2]}
	importAvailabilityRecords(t, owner, peers)
	return availabilityFixture{now: now, owner: owner, ownerDir: dirs[3], blob: blob, key: key, plaintext: plaintext, peers: peers, peerDirs: peerDirs, bootstrapPeer: principals[0]}
}

func availabilityNodeConfig(name, dir string, privacy, dataPrivacy *networkprivacy.Channel, bootstrap, anchors []string) runtimeprocess.Config {
	return runtimeprocess.Config{
		Name: name, NodeProfile: networkapi.NodeProfileServiceNode,
		Boot:      runtimeprocess.BootConfig{Sources: bootstrap},
		Transport: runtimeprocess.TransportConfig{BindAddress: "127.0.0.1", ReachabilityMode: networkapi.ReachabilityPrivateLAN},
		Data:      runtimeprocess.DataConfig{Dir: dir, MaxRelayRetentionBytes: 1024 * 1024},
		Trust:     runtimeprocess.TrustConfig{Anchors: anchors}, Privacy: privacy, DataPrivacy: dataPrivacy,
		DiscoveryRefreshInterval: 50 * time.Millisecond,
	}
}

func importAvailabilityRecords(t *testing.T, owner runtimeprocess.NodeRuntime, peers map[string]runtimeprocess.NodeRuntime) {
	t.Helper()
	filter := func(record discoveryapi.DiscoveryRecord) bool { return record.Kind == "node" }
	for principal, peer := range peers {
		testkit.ImportRecordsFromNode(t, owner, peer, "availability-peer", filter)
		testkit.ImportRecordsFromNode(t, peer, owner, "availability-owner", filter)
		require.NotEmpty(t, principal)
	}
}

func publishAvailabilityManifest(t *testing.T, fixture availabilityFixture) dataapi.ManifestSnapshot {
	t.Helper()
	root, err := fixture.owner.PublishManifest(dataapi.ManifestSnapshot{
		Kind: "blob-set", Owner: fixture.owner.Snapshot().Ident.Principal, Encrypted: true, Retention: "durable",
		Refs: []dataapi.RefSnapshot{{Kind: "blob", ID: fixture.blob.ID}},
	})
	require.NoError(t, err)
	return root
}

func setDurableIntent(t *testing.T, owner runtimeprocess.NodeRuntime, rootID string, now time.Time) {
	t.Helper()
	_, err := owner.SetReplicaIntent(dataapi.ReplicaIntentSnapshot{
		ID: "availability-e2e-intent", RootManifestID: rootID, Version: 1,
		DesiredCopies: 3, MinimumCopies: 2, LeaseDuration: 24 * time.Hour,
		RenewalHorizon: 8 * time.Hour, Retention: "durable", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
}

func assertAvailability(t *testing.T, owner runtimeprocess.NodeRuntime, rootID, state string, valid int) {
	t.Helper()
	snapshot, err := owner.GetAvailability(rootID)
	require.NoError(t, err)
	require.Equal(t, state, snapshot.State)
	require.Equal(t, valid, snapshot.ValidCopies)
}

func activeRemoteCommitments(state dataplacement.State) []dataplacement.Commitment {
	out := make([]dataplacement.Commitment, 0, len(state.Commitments))
	for _, commitment := range state.Commitments {
		if commitment.State == dataplacement.CommitmentActive {
			out = append(out, commitment)
		}
	}
	return out
}

func commitmentPeers(items []dataplacement.Commitment) []string {
	peers := make([]string, 0, len(items))
	for _, item := range items {
		peers = append(peers, item.PeerID)
	}
	return peers
}

func lossCommitment(items []dataplacement.Commitment, bootstrapPeer string) dataplacement.Commitment {
	for _, item := range items {
		if item.PeerID != bootstrapPeer {
			return item
		}
	}
	return dataplacement.Commitment{}
}

func assertForeignCopiesAreCiphertext(t *testing.T, fixture availabilityFixture, commitments []dataplacement.Commitment) {
	t.Helper()
	for _, commitment := range commitments {
		raw, err := os.ReadFile(e2eReplicaPayloadPath(fixture.peerDirs[commitment.PeerID], fixture.blob.ID))
		require.NoError(t, err)
		require.NotEqual(t, fixture.plaintext, raw)
		require.False(t, bytes.Contains(raw, fixture.plaintext))
		stored, err := fixture.peers[commitment.PeerID].GetBlob(fixture.blob.ID)
		require.NoError(t, err)
		require.True(t, stored.Encrypted)
		require.Empty(t, stored.Payload)
	}
}

func e2eReplicaPayloadPath(dir, id string) string {
	safeID := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(id)
	return filepath.Join(dir, "blobs", safeID+".blob")
}
