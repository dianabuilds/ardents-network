//go:build e2e

package replication_e2e_test

import (
	"context"
	"os"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeprocess "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	networkprivacy "ardents/internal/messaging"
	appreplication "ardents/internal/replication"
	"ardents/internal/replication/availability"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestAvailabilityFailureMatrixEndsInHonestTerminalLoss(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerE2E, Domain: "data-substrate", ScenarioID: "DAE-003",
		Suite: "e2e", Tags: []string{"integration", "e2e", "data-substrate", "quota", "revocation", "corruption"},
		Speed: "default", Environment: "local",
	})
	fixture := startAvailabilityFailureFixture(t)
	root := publishFailureManifest(t, fixture)
	setFailureIntent(t, fixture.owner, root.ID, fixture.now)
	ownerNode := fixture.owner
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	outcome, err := runtimeprocess.PlaceAvailableBlobReplicasForIntegrationTest(ownerNode, ctx, fixture.blob.ID, 1, 1)
	require.NoError(t, err)
	require.Len(t, outcome.Commitments, 1)
	require.Contains(t, outcome.Decision.Denials, appreplication.ReplicaPlacementDenial{NodeID: fixture.quotaPeer, Reason: appreplication.ReplicaReasonQuota})
	require.NoError(t, fixture.owner.ReconcileDataAvailability(ctx))
	assertAvailability(t, fixture.owner, root.ID, "target-satisfied", 2)

	commitment := outcome.Commitments[0]
	spare := fixture.healthySpare(commitment.PeerID)
	require.NotEmpty(t, spare)
	require.NoError(t, os.WriteFile(e2eReplicaPayloadPath(fixture.peerDirs[commitment.PeerID], fixture.blob.ID), []byte("corrupt replica"), 0o600))
	corrupt, err := runtimeprocess.ProbeBlobReplicaForIntegrationTest(ownerNode, ctx, commitment)
	require.Error(t, err)
	require.Equal(t, appreplication.ReplicaCommitmentCorrupt, corrupt.State)
	fixture.dataPrivacy.RevokeSender(t, fixture.peerIndexes[spare], fixture.ownerIndex, fixture.now)
	_, err = fixture.owner.DropBlob(fixture.blob.ID)
	require.NoError(t, err)

	err = fixture.owner.ReconcileDataAvailability(ctx)
	require.Error(t, err)
	require.ErrorContains(t, err, appreplication.ReplicaReasonCapability)
	unavailable, err := fixture.owner.GetAvailability(root.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", unavailable.State)
	require.Zero(t, unavailable.ValidCopies)
	require.Equal(t, 1, unavailable.CorruptCopies)
	require.Equal(t, 2, unavailable.PendingRepairs)

	advanceRepairsToTerminal(t, ownerNode, fixture.owner.ListReplicaRepairs(root.ID))
	require.NoError(t, fixture.owner.ReconcileDataAvailability(ctx))
	lost, err := fixture.owner.GetAvailability(root.ID)
	require.NoError(t, err)
	require.Equal(t, "lost", lost.State)
	require.Zero(t, lost.ValidCopies)
	require.Zero(t, lost.CurrentLeases)
	require.Zero(t, lost.PendingRepairs)
	for _, repair := range fixture.owner.ListReplicaRepairs(root.ID) {
		require.Equal(t, "failed", repair.State)
		require.Equal(t, 6, repair.PostLeaseAttempts)
	}
	events, _ := testkit.Diagnostics(fixture.owner).ListRecentEvents(100, "")
	require.True(t, containsDataEvent(events, "availability_observed"))
}

type availabilityFailureFixture struct {
	now         time.Time
	owner       *runtimeprocess.Node
	ownerIndex  int
	blob        appdata.Blob
	quotaPeer   string
	peers       map[string]*runtimeprocess.Node
	peerDirs    map[string]string
	peerIndexes map[string]int
	dataPrivacy testkit.PrivacyGroupFixture
}

func (f availabilityFailureFixture) healthySpare(committedPeer string) string {
	for peer := range f.peers {
		if peer != f.quotaPeer && peer != committedPeer {
			return peer
		}
	}
	return ""
}

func startAvailabilityFailureFixture(t *testing.T) availabilityFailureFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	discoveryPrivacy := testkit.NewDiscoveryPrivacyGroupFixture(t, now, 4)
	dataPrivacy := testkit.NewDataPrivacyGroupFixture(t, now, 4)
	dirs := []string{t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()}
	names := []string{"failure-quota", "failure-peer-two", "failure-peer-three", "failure-owner"}
	seeds, identities, principals := seedFailureNodes(t, names, dirs, discoveryPrivacy, dataPrivacy)
	for _, seed := range seeds {
		require.NoError(t, seed.Stop(context.Background()))
	}
	store := appdata.NewInDir(dirs[3])
	require.NoError(t, store.Load())
	blob, err := store.StoreEncryptedBlob(appdata.Blob{MediaType: "application/octet-stream", Retention: "durable"}, []byte("failure matrix owner plaintext"), []byte("0123456789abcdef0123456789abcdef"), "failure-owner-key")
	require.NoError(t, err)

	peerOne := testkit.StartNode(t, failureNodeConfig(names[0], dirs[0], discoveryPrivacy.Channels[0], dataPrivacy.Channels[0], []string{"local://bootstrap"}, []string{identities[3]}, 1))
	peerTwo := testkit.StartNode(t, failureNodeConfig(names[1], dirs[1], discoveryPrivacy.Channels[1], dataPrivacy.Channels[1], testkit.BootstrapEndpoints(t, peerOne), []string{identities[3]}, 0))
	peerThreeBootstrap := append(testkit.BootstrapEndpoints(t, peerOne), testkit.BootstrapEndpoints(t, peerTwo)...)
	peerThree := testkit.StartNode(t, failureNodeConfig(names[2], dirs[2], discoveryPrivacy.Channels[2], dataPrivacy.Channels[2], peerThreeBootstrap, []string{identities[3]}, 0))
	ownerBootstrap := append(testkit.BootstrapEndpoints(t, peerOne), append(testkit.BootstrapEndpoints(t, peerTwo), testkit.BootstrapEndpoints(t, peerThree)...)...)
	owner := testkit.StartNode(t, failureNodeConfig(names[3], dirs[3], discoveryPrivacy.Channels[3], dataPrivacy.Channels[3], ownerBootstrap, identities[:3], 0))
	peers := map[string]*runtimeprocess.Node{principals[0]: peerOne, principals[1]: peerTwo, principals[2]: peerThree}
	peerDirs := map[string]string{principals[0]: dirs[0], principals[1]: dirs[1], principals[2]: dirs[2]}
	peerIndexes := map[string]int{principals[0]: 0, principals[1]: 1, principals[2]: 2}
	importAvailabilityRecords(t, owner, peers)
	return availabilityFailureFixture{now: now, owner: owner, ownerIndex: 3, blob: blob, quotaPeer: principals[0], peers: peers, peerDirs: peerDirs, peerIndexes: peerIndexes, dataPrivacy: dataPrivacy}
}

func seedFailureNodes(t *testing.T, names, dirs []string, discoveryPrivacy, dataPrivacy testkit.PrivacyGroupFixture) ([]*runtimeprocess.Node, []string, []string) {
	t.Helper()
	seeds := make([]*runtimeprocess.Node, 4)
	identities := make([]string, 4)
	principals := make([]string, 4)
	for index := range seeds {
		seeds[index] = testkit.StartNode(t, failureNodeConfig(names[index], dirs[index], discoveryPrivacy.Channels[index], dataPrivacy.Channels[index], []string{"local://bootstrap"}, nil, 0))
		identities[index] = seeds[index].Snapshot().Ident.PublicKey
		principals[index] = seeds[index].Snapshot().Ident.Principal
	}
	return seeds, identities, principals
}

func failureNodeConfig(name, dir string, privacy, dataPrivacy *networkprivacy.Channel, bootstrap, anchors []string, quota int64) runtimeprocess.Config {
	cfg := availabilityNodeConfig(name, dir, privacy, dataPrivacy, bootstrap, anchors)
	cfg.Data.MaxReplicaRetentionBytes = quota
	return cfg
}

func publishFailureManifest(t *testing.T, fixture availabilityFailureFixture) appdata.Manifest {
	t.Helper()
	root, err := fixture.owner.PublishManifest(appdata.Manifest{
		Kind: "blob-set", Owner: fixture.owner.Snapshot().Ident.Principal, Encrypted: true, Retention: "durable",
		Refs: []appdata.Ref{{Kind: "blob", ID: fixture.blob.ID}},
	})
	require.NoError(t, err)
	return root
}

func setFailureIntent(t *testing.T, owner *runtimeprocess.Node, rootID string, now time.Time) {
	t.Helper()
	_, err := owner.SetReplicaIntent(availability.ReplicaIntent{
		ID: "availability-failure-intent", RootManifestID: rootID, Version: 1,
		DesiredCopies: 2, MinimumCopies: 1, LeaseDuration: 24 * time.Hour,
		RenewalHorizon: 8 * time.Hour, Retention: "durable", CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)
}

func advanceRepairsToTerminal(t *testing.T, ownerNode *runtimeprocess.Node, repairs []availability.RepairRecord) {
	t.Helper()
	for _, snapshot := range repairs {
		next := snapshot.NextAttemptAt
		attempts := snapshot.PostLeaseAttempts
		for attempts < 6 {
			repair, err := runtimeprocess.RecordRepairFailureForIntegrationTest(ownerNode, snapshot.ID, next)
			require.NoError(t, err)
			next = repair.NextAttemptAt
			attempts = repair.PostLeaseAttempts
		}
	}
}

func containsDataEvent(events []diagapi.EventEnvelope, eventType string) bool {
	for _, event := range events {
		if event.Domain == "data" && event.Type == eventType {
			return true
		}
	}
	return false
}
